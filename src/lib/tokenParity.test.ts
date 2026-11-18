import { describe, expect, it } from 'vitest';
import { tokenize } from './search';
// Cross-language parity fixture, owned by the Go index
// (internal/index/fixtures_test.go, regenerate with UPDATE_TOKEN_FIXTURES=1).
// If the two tokenizers drift, this test fails on the exact input.
import rows from '../../internal/index/testdata/tokens.json';

type Row = { input: string; tokens: string[] };

const cases = rows as Row[];

describe('tokenize parity with the Go index', () => {
  it('has fixture rows', () => {
    expect(cases.length).toBeGreaterThan(20);
  });

  it.each(cases.map((row, index) => [index, row] as const))(
    'row %i matches the Go tokenizer',
    (_index, row) => {
      expect(tokenize(row.input)).toEqual(row.tokens);
    },
  );
});
