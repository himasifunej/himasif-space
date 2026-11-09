#!/usr/bin/env python3
"""Compare explicitly exported Reo jobdesk rows with a Space emergency CSV.
Read-only: no Google, WA, SQL, source writes, title-based pairing or H-2 math.
"""
import argparse
import csv
import datetime as dt
from decimal import Decimal, InvalidOperation
import hashlib
import json
import os
from pathlib import Path
import re
import sys
import unicodedata
import uuid


class ComparisonError(Exception):
    pass


def require(condition, message):
    if not condition:
        raise ComparisonError(message)


def digest(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def rows(path, maximum=5000):
    require(Path(path).stat().st_size <= 32 * 1024**2, 'CSV exceeds 32 MiB')
    with Path(path).open(encoding='utf-8-sig', newline='') as source:
        reader = csv.DictReader(source)
        require(reader.fieldnames and len(set(reader.fieldnames)) == len(reader.fieldnames), 'missing/duplicate CSV headers')
        result = []
        for row in reader:
            require(None not in row and all(value is not None for value in row.values()), 'CSV column count mismatch')
            result.append(row)
            require(len(result) <= maximum, 'CSV exceeds the row limit')
        return result, reader.fieldnames


def normalized(value):
    return ' '.join(unicodedata.normalize('NFKC', value).casefold().split())


def legacy_date(value, encoding):
    value = value.strip()
    if encoding == 'iso_date':
        require(re.fullmatch(r'\d{4}-\d{2}-\d{2}', value), 'legacy deadline is not an explicit ISO date')
        return dt.date.fromisoformat(value).isoformat()
    if encoding == 'google_serial':
        serial = Decimal(value)
        require(serial.is_finite() and serial == serial.to_integral_value(), 'legacy date has missing/ambiguous clock precision')
        return (dt.date(1899, 12, 30) + dt.timedelta(days=int(serial))).isoformat()
    raise ComparisonError('unsupported date_encoding; explicitly normalize legacy text dates before comparing')


def spreadsheet_text(value):
    value = str(value)
    return "'" + value if value.lstrip().startswith(('=', '+', '-', '@')) else value


def main():
    os.umask(0o077)
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--legacy', required=True)
    parser.add_argument('--space', required=True)
    parser.add_argument('--bindings', required=True)
    parser.add_argument('--mapping', required=True)
    parser.add_argument('--output', required=True)
    args = parser.parse_args()
    mapping = json.loads(Path(args.mapping).read_text(encoding='utf-8'))
    require(mapping.get('date_semantics') == 'already_design_deadline',
            'this comparator requires an existing design deadline; it never subtracts H-2 from Reo jobdesk again')
    columns = mapping.get('columns', {})
    require(set(columns) == {'key', 'title', 'deadline', 'status', 'approval'}, 'declare all five legacy column names')
    require(mapping.get('date_encoding') in ('iso_date', 'google_serial'), 'declare date_encoding explicitly')
    status_map = mapping.get('status_progress', {})
    for states in status_map.values():
        require(isinstance(states, list) and states and set(states) <= {'todo', 'doing', 'submitted', 'done', 'cancelled'}, 'invalid explicit status mapping')
    legacy, legacy_headers = rows(args.legacy)
    space, space_headers = rows(args.space)
    bindings, binding_headers = rows(args.bindings)
    require(set(columns.values()) <= set(legacy_headers), 'mapped legacy columns are missing')
    require({'task_id', 'title', 'deadline_kind', 'deadline_value', 'base_progress', 'base_match_status', 'snapshot_id', 'captured_at_utc'} <= set(space_headers), 'not a Space emergency export')
    require({'legacy_key', 'task_id'} <= set(binding_headers), 'bindings require legacy_key and task_id')
    output = Path(args.output)
    require(output.is_absolute() and not output.exists(), 'comparison output must be a new absolute directory')
    tasks = {}
    for row in space:
        task_id = str(uuid.UUID(row['task_id']))
        require(task_id not in tasks, 'Space export has duplicate task IDs')
        tasks[task_id] = row
    bound = {}
    target_bindings = {}
    for row in bindings:
        key = row['legacy_key'].strip()
        require(key and key not in bound, 'duplicate or empty explicit legacy binding')
        task_id = str(uuid.UUID(row['task_id']))
        bound[key] = task_id
        target_bindings.setdefault(task_id, []).append(key)
    issues = []
    compared = []
    keys = set()
    def issue(code, key, task_id='', detail=''):
        issues.append({'code': code, 'legacyKey': key, 'taskId': task_id, 'detail': detail})
    for number, row in enumerate(legacy, start=2):
        key = row[columns['key']].strip()
        title = row[columns['title']].strip()
        raw_deadline = row[columns['deadline']].strip()
        if not key and not title and not raw_deadline:
            continue
        if not key:
            issue('missing_legacy_key', '', detail=f'CSV row {number}; do not pair from title alone')
            continue
        if key in keys:
            issue('duplicate_legacy_key', key)
            continue
        keys.add(key)
        task_id = bound.get(key)
        if not task_id:
            issue('unbound', key)
            continue
        if len(target_bindings[task_id]) != 1:
            issue('multiple_legacy_rows_for_task', key, task_id)
        current = tasks.get(task_id)
        if current is None:
            issue('target_missing_from_export', key, task_id)
            continue
        before_count = len(issues)
        if normalized(title) != normalized(current['title']):
            issue('title_changed', key, task_id, title + ' | ' + current['title'])
        try:
            date = legacy_date(raw_deadline, mapping['date_encoding'])
            if current['deadline_kind'] != 'date':
                issue('deadline_precision_requires_review', key, task_id)
            elif date != current['deadline_value']:
                issue('deadline_changed', key, task_id, date + ' | ' + current['deadline_value'])
        except (ComparisonError, ValueError, InvalidOperation, OverflowError):
            issue('legacy_date_requires_review', key, task_id)
        status = normalized(row[columns['status']])
        allowed = status_map.get(status)
        if not allowed:
            issue('unmapped_status', key, task_id, status)
        elif current['base_progress'] not in allowed:
            issue('status_differs', key, task_id, status + ' | ' + current['base_progress'])
        if status in set(mapping.get('completion_statuses', ['done'])):
            issue('legacy_completion_is_not_verified_evidence', key, task_id,
                  'review actual submission/evidence separately; do not backdate Space evidence from a done click')
        if row[columns['approval']].strip():
            issue('legacy_approval_requires_review', key, task_id,
                  'approval meaning and provenance must be reviewed; it is not automatically publication or SOP confirmation')
        compared.append({'legacyKey': key, 'taskId': task_id,
                         'fieldComparison': 'matched' if len(issues) == before_count else 'requires_review',
                         'spaceMatchStatus': current['base_match_status']})
    for key, task_id in bound.items():
        if key not in keys:
            issue('bound_legacy_key_missing', key, task_id, 'a missing source row is not an instruction to delete a Space task')
    bound_targets = set(bound.values())
    for task_id in tasks:
        if task_id not in bound_targets:
            issue('space_task_without_legacy_binding', '', task_id,
                  'may be app-native; classify explicitly rather than guessing a source match')
    report = {'generatedAt': dt.datetime.now(dt.timezone.utc).isoformat(),
              'inputSHA256': {name: digest(getattr(args, name)) for name in ('legacy', 'space', 'bindings', 'mapping')},
              'spaceSnapshots': sorted(set(row['snapshot_id'] for row in space)),
              'spaceCapturedAt': sorted(set(row['captured_at_utc'] for row in space)),
              'legacyRows': len(keys), 'spaceTasks': len(tasks), 'compared': compared,
              'issues': issues, 'needsReview': bool(issues), 'cutoverAuthorized': False,
              'externalWrites': False, 'deadlineOffsetApplied': False}
    output.mkdir(mode=0o700, parents=True)
    (output / 'comparison.json').write_text(json.dumps(report, ensure_ascii=False, indent=2) + '\n', encoding='utf-8')
    with (output / 'review.csv').open('w', encoding='utf-8', newline='') as target:
        writer = csv.writer(target)
        writer.writerow(['code', 'legacy_key', 'task_id', 'detail', 'reviewer', 'decision', 'reason', 'space_reference'])
        for item in issues:
            writer.writerow([spreadsheet_text(item['code']), spreadsheet_text(item['legacyKey']),
                             spreadsheet_text(item['taskId']), spreadsheet_text(item['detail']), '', '', '', ''])
    print(json.dumps({'report': str(output / 'comparison.json'), 'issues': len(issues), 'cutoverAuthorized': False}))


if __name__ == '__main__':
    try:
        main()
    except (ComparisonError, OSError, ValueError, KeyError, TypeError) as error:
        print('comparison: ' + (str(error) if isinstance(error, ComparisonError) else type(error).__name__), file=sys.stderr)
        sys.exit(1)
