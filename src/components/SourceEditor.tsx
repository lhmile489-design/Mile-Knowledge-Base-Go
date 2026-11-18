import { useEffect, useRef } from "react";
import { EditorState } from "@codemirror/state";
import { EditorView, keymap, lineNumbers, highlightActiveLine, drawSelection } from "@codemirror/view";
import { defaultKeymap, history, historyKeymap, indentWithTab } from "@codemirror/commands";
import { searchKeymap, highlightSelectionMatches } from "@codemirror/search";
import { markdown } from "@codemirror/lang-markdown";

type Props = {
  path: string;
  value: string;
  readOnly: boolean;
  onChange: (text: string) => void;
  onSave: () => void;
};

/** Source mode: monospace with line numbers, no syntax decoration beyond the
 * Markdown structure (DESIGN.md §5.8). External value changes are applied as a
 * minimal diff, never as a whole-document replace. */
export function SourceEditor({ path, value, readOnly, onChange, onSave }: Props) {
  const hostRef = useRef<HTMLDivElement | null>(null);
  const viewRef = useRef<EditorView | null>(null);
  const onChangeRef = useRef(onChange);
  const onSaveRef = useRef(onSave);
  onChangeRef.current = onChange;
  onSaveRef.current = onSave;

  // One editor per document; switching tabs recreates it.
  useEffect(() => {
    const host = hostRef.current;
    if (!host) return;
    const view = new EditorView({
      parent: host,
      state: EditorState.create({
        doc: value,
        extensions: [
          lineNumbers(),
          highlightActiveLine(),
          drawSelection(),
          history(),
          EditorView.lineWrapping,
          highlightSelectionMatches(),
          markdown(),
          keymap.of([
            {
              key: "Mod-s",
              preventDefault: true,
              run: () => {
                onSaveRef.current();
                return true;
              },
            },
            ...defaultKeymap,
            ...historyKeymap,
            ...searchKeymap,
            indentWithTab,
          ]),
          EditorView.updateListener.of((update) => {
            if (update.docChanged) onChangeRef.current(update.state.doc.toString());
          }),
          EditorView.editable.of(!readOnly),
        ],
      }),
    });
    viewRef.current = view;
    return () => {
      view.destroy();
      viewRef.current = null;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [path, readOnly]);

  // Apply outside changes (adopt-disk, conflict resolution) as a minimal edit.
  useEffect(() => {
    const view = viewRef.current;
    if (!view) return;
    const current = view.state.doc.toString();
    if (current === value) return;
    const { from, to, insert } = minimalChange(current, value);
    view.dispatch({ changes: { from, to, insert } });
  }, [value]);

  return <div className="source-editor-host" ref={hostRef} data-path={path} />;
}

/** Common prefix/suffix diff: enough to keep the cursor and history sane when
 * the app itself replaces the document. */
export function minimalChange(current: string, next: string): { from: number; to: number; insert: string } {
  const max = Math.min(current.length, next.length);
  let prefix = 0;
  while (prefix < max && current.charCodeAt(prefix) === next.charCodeAt(prefix)) prefix += 1;
  let suffix = 0;
  while (
    suffix < max - prefix &&
    current.charCodeAt(current.length - 1 - suffix) === next.charCodeAt(next.length - 1 - suffix)
  ) {
    suffix += 1;
  }
  return {
    from: prefix,
    to: current.length - suffix,
    insert: next.slice(prefix, next.length - suffix),
  };
}
