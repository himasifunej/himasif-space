package config

import (
	"strings"
	"testing"
)

func TestMailRateAndSNSTopic(t *testing.T) {
	t.Setenv("SPACE_DATABASE_URL", "postgres://x")
	c, err := Load()
	if err != nil || c.Mail.PerMinute != 20 || c.Mail.SNSTopicARN != "" {
		t.Fatalf("defaults %+v %v", c.Mail, err)
	}
	t.Setenv("SPACE_MAIL_DRIVER", "disabled")
	t.Setenv("SPACE_MAIL_PER_MINUTE", "60")
	t.Setenv("SPACE_SES_SNS_TOPIC_ARN", "arn:aws:sns:ap-southeast-2:123456789012:space-ses-events")
	if c, err = Load(); err != nil || c.Mail.Driver != "disabled" || c.Mail.PerMinute != 60 {
		t.Fatalf("set %+v %v", c.Mail, err)
	}
	t.Setenv("SPACE_MAIL_PER_MINUTE", "0")
	t.Setenv("SPACE_SES_SNS_TOPIC_ARN", "arn:aws:sqs:ap-southeast-2:123456789012:queue")
	_, err = Load()
	if err == nil || !strings.Contains(err.Error(), "SPACE_MAIL_PER_MINUTE") || !strings.Contains(err.Error(), "SPACE_SES_SNS_TOPIC_ARN") {
		t.Fatalf("invalid = %v", err)
	}
}
