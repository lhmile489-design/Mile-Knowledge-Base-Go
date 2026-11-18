import { describe, expect, it } from 'vitest';
import { tokenize } from './search';

// Ported from tests/unit/tokenize.test.ts (legacy v2.7.0, 21 cases). The legacy
// file also covers parseDocument, which lives in Go now (internal/index); its
// cases are ported in internal/index/tokenize_test.go.

describe('tokenize', () => {
  it('splits english on word boundaries and lowercases', () => {
    expect(tokenize('Hello World')).toContain('hello');
    expect(tokenize('Hello World')).toContain('world');
    expect(tokenize('Hello World')).not.toContain('Hello');
  });

  it('keeps digits and version-like tokens together', () => {
    const tokens = tokenize('deepseek v4.1 release');
    expect(tokens).toContain('deepseek');
    expect(tokens).toContain('v4.1');
    expect(tokens).toContain('release');
  });

  it('does not split camel case into separate words', () => {
    expect(tokenize('getUserName')).toEqual(['getusername']);
  });

  it('emits cjk bigrams so partial chinese phrases still match', () => {
    const tokens = tokenize('知识库管理');
    expect(tokens).toContain('知识');
    expect(tokens).toContain('识库');
    expect(tokens).toContain('库管');
    expect(tokens).toContain('管理');
  });

  it('emits a single-character token for a lone cjk character', () => {
    expect(tokenize('中')).toEqual(['中']);
  });

  it('handles mixed chinese and english in one string', () => {
    const tokens = tokenize('React 状态管理 best practice');
    expect(tokens).toContain('react');
    expect(tokens).toContain('状态');
    expect(tokens).toContain('态管');
    expect(tokens).toContain('best');
    expect(tokens).toContain('practice');
  });

  it('treats punctuation and whitespace as separators', () => {
    expect(tokenize('alpha, beta; gamma\n\tdelta')).toEqual(['alpha', 'beta', 'gamma', 'delta']);
  });

  it('does not bridge a bigram across a punctuation boundary', () => {
    expect(tokenize('知识库。管理')).not.toContain('库管');
  });

  it('returns an empty list for empty or symbol-only input', () => {
    expect(tokenize('')).toEqual([]);
    expect(tokenize('!!! ---')).toEqual([]);
  });

  it('deduplicates repeated tokens', () => {
    expect(tokenize('test test test').filter((t) => t === 'test')).toHaveLength(1);
  });

  it('truncates absurdly long input instead of hanging', () => {
    const tokens = tokenize('a'.repeat(2_000_000));
    expect(tokens.length).toBeGreaterThan(0);
    expect(tokens.length).toBeLessThan(500_000);
  });
});
