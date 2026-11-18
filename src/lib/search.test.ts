import { describe, expect, it } from 'vitest';
import {
  describeMatchedFields,
  highlightRanges,
  MATCH_FIELD_LABEL,
  MATCH_FIELDS,
  tokenize,
} from './search';

// Ported from tests/unit/searchHighlight.test.ts (legacy v2.7.0, 9 cases).

describe('describeMatchedFields', () => {
  it('maps every internal enum to a chinese label', () => {
    for (const field of MATCH_FIELDS) {
      expect(MATCH_FIELD_LABEL[field].length).toBeGreaterThan(0);
      expect(MATCH_FIELD_LABEL[field]).not.toBe(field);
    }
  });

  it('joins fields with the chinese comma', () => {
    expect(describeMatchedFields(['title', 'body'])).toBe('标题、正文');
  });

  it('keeps order and repeats nothing', () => {
    expect(describeMatchedFields(['tag'])).toBe('标签');
    expect(describeMatchedFields([])).toBe('');
  });
});

describe('highlightRanges', () => {
  it('finds single and multiple token matches case-insensitively', () => {
    expect(highlightRanges('Hello World', ['hello'])).toEqual([{ start: 0, end: 5 }]);
    expect(highlightRanges('Hello hello world', ['hello'])).toEqual([
      { start: 0, end: 5 },
      { start: 6, end: 11 },
    ]);
  });

  it('merges overlapping ranges from different tokens', () => {
    const tokens = tokenize('知识库');
    const ranges = highlightRanges('这是一段知识库文本', tokens);
    expect(ranges).toHaveLength(1);
    expect(ranges[0]).toEqual({ start: 4, end: 7 });
  });

  it('returns ranges in order without gaps or inversions', () => {
    const ranges = highlightRanges('abc def abc', ['abc', 'def']);
    for (let i = 1; i < ranges.length; i += 1) {
      expect(ranges[i]!.start).toBeGreaterThanOrEqual(ranges[i - 1]!.end);
    }
    expect(ranges.map((r) => 'abc def abc'.slice(r.start, r.end))).toEqual(['abc', 'def', 'abc']);
  });

  it('returns empty for no match or empty input', () => {
    expect(highlightRanges('nothing here', ['zzz'])).toEqual([]);
    expect(highlightRanges('', ['a'])).toEqual([]);
    expect(highlightRanges('text', [])).toEqual([]);
  });

  it('never treats the query as a regex', () => {
    expect(highlightRanges('the (a+)+$ pattern', ['(a+)+$'])).toEqual([{ start: 4, end: 10 }]);
  });

  it('uses the same tokenizer as the index so bigram tokens align', () => {
    const text = '受控组件与状态提升';
    const ranges = highlightRanges(text, tokenize('状态'));
    expect(ranges.length).toBeGreaterThan(0);
    for (const range of ranges) {
      expect(text.slice(range.start, range.end)).toBe('状态');
    }
  });
});
