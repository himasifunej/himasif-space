#!/usr/bin/env python3
"""Checks contributors.json against format 1 (docs/contributors.md).

Standard library only, so it runs in CI and on any contributor's machine:
    python3 scripts/check_contributors.py [path]
"""
import json
import re
import sys

KINDS = {'lead', 'code', 'design', 'docs', 'test', 'release', 'ops',
         'content', 'translation', 'idea', 'pilot'}
FIELDS = {'id', 'name', 'alias', 'role', 'kind', 'affiliation', 'period',
          'github', 'links'}
ID = re.compile(r'^[a-z0-9][a-z0-9-]{0,38}$')
GITHUB = re.compile(r'^[A-Za-z0-9](?:[A-Za-z0-9-]{0,37}[A-Za-z0-9])?$')
PERIOD = re.compile(r'^[0-9]{4}/[0-9]{4}$')
PRIVATE = re.compile(r'@[\w-]+\.[\w.]+|(?:\+62|\b08)\d{8,}')


def check(doc):
    errs = []
    if set(doc) - {'$schema', 'format', 'project', 'contributors'}:
        errs.append(f'unknown top-level fields: {sorted(set(doc) - {"$schema", "format", "project", "contributors"})}')
    if doc.get('format') != 1:
        errs.append('format must be 1')
    people = doc.get('contributors')
    if not isinstance(people, list) or not people:
        return errs + ['contributors must be a non-empty array']
    seen = set()
    for i, c in enumerate(people):
        at = f'contributors[{i}]'
        if not isinstance(c, dict):
            errs.append(f'{at}: not an object')
            continue
        at = f'{at} ({c.get("id", "?")})'
        for f in sorted(set(c) - FIELDS):
            errs.append(f'{at}: unknown field {f!r}')
        for f in ('id', 'name', 'role', 'kind'):
            if f not in c:
                errs.append(f'{at}: missing {f!r}')
        cid = c.get('id', '')
        if not isinstance(cid, str) or not ID.match(cid):
            errs.append(f'{at}: id must match {ID.pattern}')
        elif cid in seen:
            errs.append(f'{at}: duplicate id')
        seen.add(cid)
        for f, limit in (('name', 80), ('alias', 40), ('role', 60), ('affiliation', 80)):
            v = c.get(f)
            if v is not None and (not isinstance(v, str) or not v.strip() or len(v) > limit):
                errs.append(f'{at}: {f} must be a non-empty string of at most {limit} characters')
        kind = c.get('kind')
        if not isinstance(kind, list) or not kind or len(set(kind)) != len(kind) or not set(kind) <= KINDS:
            errs.append(f'{at}: kind must be a non-empty list of distinct values from {sorted(KINDS)}')
        if 'period' in c and not PERIOD.match(str(c['period'])):
            errs.append(f'{at}: period must be YYYY/YYYY')
        if 'github' in c and not GITHUB.match(str(c['github'])):
            errs.append(f'{at}: github must be a GitHub username without @')
        links = c.get('links', [])
        if not isinstance(links, list) or len(links) > 4:
            errs.append(f'{at}: links must be a list of at most 4')
            links = []
        for j, l in enumerate(links):
            if not isinstance(l, dict) or set(l) != {'label', 'url'}:
                errs.append(f'{at}: links[{j}] must have exactly label and url')
            elif not str(l['url']).startswith('https://') or not 0 < len(str(l['label'])) <= 24:
                errs.append(f'{at}: links[{j}] needs an https:// url and a label of at most 24 characters')
        if PRIVATE.search(json.dumps(c, ensure_ascii=False).replace('https://', '')):
            errs.append(f'{at}: looks like an email address or phone number; the file is public')
    return errs


def main():
    path = sys.argv[1] if len(sys.argv) > 1 else 'contributors.json'
    try:
        with open(path, encoding='utf-8') as f:
            doc = json.load(f)
    except (OSError, json.JSONDecodeError) as e:
        print(f'{path}: {e}', file=sys.stderr)
        return 1
    errs = check(doc) if isinstance(doc, dict) else ['top level must be an object']
    for e in errs:
        print(f'{path}: {e}', file=sys.stderr)
    if not errs:
        print(f'{path}: {len(doc["contributors"])} contributors, format 1, ok')
    return 1 if errs else 0


if __name__ == '__main__':
    sys.exit(main())
