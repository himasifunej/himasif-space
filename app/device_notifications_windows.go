//go:build windows

package main

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/himasifunej/himasif-space/internal/api"
	"golang.org/x/sys/windows/registry"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"

	"github.com/egoist/mygo"
)

func openNotificationSettings() error { return mygo.Shell.OpenExternal("ms-settings:notifications") }

func powershell(script string) error {
	words := utf16.Encode([]rune(script))
	b := make([]byte, len(words)*2)
	for i, w := range words {
		binary.LittleEndian.PutUint16(b[i*2:], w)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive", "-EncodedCommand", base64.StdEncoding.EncodeToString(b))
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	output, err := cmd.CombinedOutput()
	if err != nil {
		text := string(output)
		for code, message := range map[string]string{
			"SPACE_NOTIFICATION_SETTING=1": "Notifikasi Space dinonaktifkan di pengaturan Windows. Aktifkan notifikasi untuk Space.",
			"SPACE_NOTIFICATION_SETTING=2": "Notifikasi untuk akun Windows ini dinonaktifkan. Aktifkan notifikasi Windows.",
			"SPACE_NOTIFICATION_SETTING=3": "Notifikasi dibatasi kebijakan administrator Windows.",
			"SPACE_NOTIFICATION_SETTING=4": "Windows tidak mengenali aplikasi ini sebagai aplikasi yang mendukung notifikasi. Pasang ulang Space lalu coba lagi.",
		} {
			if strings.Contains(text, code) {
				return errors.New(message)
			}
		}
		return fmt.Errorf("Windows belum menerima notifikasi: %w", err)
	}
	return nil
}
func setDeviceAutostart(enabled bool) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Run`, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	exe := ""
	if enabled {
		exe, err = os.Executable()
		if err != nil {
			return err
		}
	}
	return writeDeviceAutostart(k, exe, enabled)
}

func writeDeviceAutostart(k registry.Key, exe string, enabled bool) error {
	if !enabled {
		err := k.DeleteValue("HIMASIF Space")
		if err == registry.ErrNotExist {
			return nil
		}
		return err
	}
	return k.SetStringValue("HIMASIF Space", `"`+exe+`" --background`)
}

// A Start Menu shortcut carries the same AUMID as the WinRT notifier.
// Protocol activation is registered by MyGo; no balloon API is used here.
func registerNotifications() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	payload, _ := json.Marshal(map[string]string{"exe": exe})
	script := `$ErrorActionPreference='Stop'
$p=[Text.Encoding]::UTF8.GetString([Convert]::FromBase64String('` + base64.StdEncoding.EncodeToString(payload) + `'))|ConvertFrom-Json
$shortcut=Join-Path ([Environment]::GetFolderPath('Programs')) 'Space.lnk'
$w=New-Object -ComObject WScript.Shell
$l=$w.CreateShortcut($shortcut);$l.TargetPath=$p.exe;$l.WorkingDirectory=[IO.Path]::GetDirectoryName($p.exe);$l.IconLocation=$p.exe+',0';$l.Save()
$legacy=Join-Path ([Environment]::GetFolderPath('Programs')) 'HIMASIF Space.lnk'
if(Test-Path -LiteralPath $legacy){$old=$w.CreateShortcut($legacy);if($old.TargetPath -eq $p.exe){Remove-Item -LiteralPath $legacy}}
Add-Type -TypeDefinition @'
using System;
using System.Runtime.InteropServices;
[StructLayout(LayoutKind.Sequential,Pack=4)] public struct PropertyKey { public Guid fmtid; public uint pid; }
[StructLayout(LayoutKind.Explicit,Size=24)] public struct PropVariant { [FieldOffset(0)] public ushort vt; [FieldOffset(8)] public IntPtr ptr; }
[ComImport,Guid("886D8EEB-8CF2-4446-8D02-CDBA1DBDCF99"),InterfaceType(ComInterfaceType.InterfaceIsIUnknown)] public interface IPropertyStore {
 void GetCount(out uint count); void GetAt(uint i,out PropertyKey key); void GetValue(ref PropertyKey key,out PropVariant value); void SetValue(ref PropertyKey key,ref PropVariant value); void Commit();
}
public static class SpaceShortcut {
 [DllImport("shell32.dll",CharSet=CharSet.Unicode,PreserveSig=false)] static extern void SHGetPropertyStoreFromParsingName(string path,IntPtr context,uint flags,ref Guid iid,[MarshalAs(UnmanagedType.Interface)] out IPropertyStore store);
 public static void SetID(string path) { Guid iid=new Guid("886D8EEB-8CF2-4446-8D02-CDBA1DBDCF99"); IPropertyStore store; SHGetPropertyStoreFromParsingName(path,IntPtr.Zero,2,ref iid,out store); PropertyKey key=new PropertyKey {fmtid=new Guid("9F4C2855-9F79-4B39-A8D0-E1D42DE1D5F3"),pid=5}; PropVariant value=new PropVariant {vt=31,ptr=Marshal.StringToCoTaskMemUni("id.himasif.space")}; try {store.SetValue(ref key,ref value);store.Commit();} finally {Marshal.FreeCoTaskMem(value.ptr);Marshal.ReleaseComObject(store);} }
}
'@
[SpaceShortcut]::SetID($shortcut)
$k='HKCU:\Software\Classes\AppUserModelId\id.himasif.space'
New-Item $k -Force | Out-Null
New-ItemProperty $k -Name DisplayName -Value 'Space — HIMASIF' -PropertyType String -Force | Out-Null
New-ItemProperty $k -Name IconUri -Value (Join-Path ([IO.Path]::GetDirectoryName($p.exe)) 'icon.png') -PropertyType String -Force | Out-Null
New-ItemProperty $k -Name SpaceExecutable -Value $p.exe -PropertyType String -Force | Out-Null
`
	return powershell(script)
}
func showDeviceNotification(d api.Delivery, link string) error {
	toast := windowsNotificationXML(d, link)
	encoded := base64.StdEncoding.EncodeToString([]byte(toast))
	group := "Space"
	if u, err := url.Parse(link); err == nil {
		group = u.Query().Get("account")
	}
	if len(group) > 64 {
		group = "Space"
	}
	return powershell(`$ErrorActionPreference='Stop'
[Windows.UI.Notifications.ToastNotificationManager,Windows.UI.Notifications,ContentType=WindowsRuntime] | Out-Null
[Windows.Data.Xml.Dom.XmlDocument,Windows.Data.Xml.Dom.XmlDocument,ContentType=WindowsRuntime] | Out-Null
$x=New-Object Windows.Data.Xml.Dom.XmlDocument
$x.LoadXml([Text.Encoding]::UTF8.GetString([Convert]::FromBase64String('` + encoded + `')))
$t=[Windows.UI.Notifications.ToastNotification]::new($x)
$t.Tag='` + windowsNotificationTag(d.ID) + `'
$t.Group='` + escapeXML(group) + `'
$n=[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier('id.himasif.space')
if($n.Setting -ne [Windows.UI.Notifications.NotificationSetting]::Enabled){[Console]::Error.WriteLine('SPACE_NOTIFICATION_SETTING='+[int]$n.Setting);exit 42}
$n.Show($t)`)
}

// showTestDeviceNotification exercises the same Windows notifier without
// creating a server delivery or offering actions for a nonexistent reminder.
func showTestDeviceNotification() error {
	return powershell(`$ErrorActionPreference='Stop'
[Windows.UI.Notifications.ToastNotificationManager,Windows.UI.Notifications,ContentType=WindowsRuntime] | Out-Null
[Windows.Data.Xml.Dom.XmlDocument,Windows.Data.Xml.Dom.XmlDocument,ContentType=WindowsRuntime] | Out-Null
$x=New-Object Windows.Data.Xml.Dom.XmlDocument
$x.LoadXml('<toast><visual><binding template="ToastGeneric"><text>Space</text><text>Notifikasi percobaan dari Space.</text></binding></visual></toast>')
$n=[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier('id.himasif.space')
if($n.Setting -ne [Windows.UI.Notifications.NotificationSetting]::Enabled){[Console]::Error.WriteLine('SPACE_NOTIFICATION_SETTING='+[int]$n.Setting);exit 42}
$n.Show([Windows.UI.Notifications.ToastNotification]::new($x))`)
}
func notificationSoundAllowed() bool {
	proc := syscall.NewLazyDLL("shell32.dll").NewProc("SHQueryUserNotificationState")
	var state uint32
	hr, _, _ := proc.Call(uintptr(unsafe.Pointer(&state)))
	// Accepting notifications (5) is the only state permitting a custom bell;
	// fullscreen, presentation, quiet time, lock screen, etc remain silent.
	if int32(hr) < 0 || state != 5 {
		return false
	}
	for _, path := range []string{`Software\Microsoft\Windows\CurrentVersion\Notifications\Settings\id.himasif.space`, `Software\Microsoft\Windows\CurrentVersion\Notifications\Settings`} {
		k, err := registry.OpenKey(registry.CURRENT_USER, path, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		sound, _, err := k.GetIntegerValue("Sound")
		k.Close()
		if err == nil && sound == 0 {
			return false
		}
	}
	return true
}

func clearDeviceNotifications(group string) {
	// Group is an account UUID supplied by the app, never external script text.
	if len(group) != 36 {
		return
	}
	_ = powershell(`[Windows.UI.Notifications.ToastNotificationManager,Windows.UI.Notifications,ContentType=WindowsRuntime] | Out-Null; [Windows.UI.Notifications.ToastNotificationManager]::History.RemoveGroup('` + group + `','id.himasif.space')`)
}
