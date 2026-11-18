import { useEffect, useRef } from "react";
import { EditorState } from "@codemirror/state";
import { EditorView, keymap, lineNumbers, highlightActiveLine, drawSelection } from "@codemirror/view";
import { defaultKeymap, history, historyKeymap, indentWithTab } from "@codemirror/commands";
import { searchKeymap, highlightSelectionMatches } from "@codemirror/search";
import { markdown } from "@codemirror/lang-markdown";
import { minimalChange } from "../lib/docUtils";
import { imageMarkdown, uploadImage } from "../lib/attachments";
import { wikilinkAt } from "../lib/wikilink";
import { useAppStore } from "../stores/appStore";

type Props = {
  path: string;
  value: string;
  readOnly: boolean;
  onChange: (text: string) => void;
  onSave: () => void;
  /** Ctrl/Cmd+click on a wikilink; returns the target text. */
  onFollowWikiLink?: (target: string) => void;
  vaultId?: string;
};

/** Source mode: monospace with line numbers, no syntax decoration beyond the
 * Markdown structure (DESIGN.md §5.8). External value changes are applied as a
 * minimal diff, never as a whole-document replace. */
export function SourceEditor({ path, value, readOnly, onChange, onSave, onFollowWikiLink, vaultId }: Props) {
  const vaultRef = useRef(vaultId);
  vaultRef.current = vaultId;
  const followRef = useRef(onFollowWikiLink);
  followRef.current = onFollowWikiLink;
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
          EditorView.domEventHandlers({
            paste: (event, view) => {
              const files = Array.from(event.clipboardData?.files ?? []).filter((file) =>
                file.type.startsWith("image/"),
              );
              if (files.length === 0 || !vaultRef.current) return false;
              event.preventDefault();
              void insertImages(view, vaultRef.current, files);
              return true;
            },
            drop: (event, view) => {
              const files = Array.from(event.dataTransfer?.files ?? []).filter((file) =>
                file.type.startsWith("image/"),
              );
              if (files.length === 0 || !vaultRef.current) return false;
              event.preventDefault();
              void insertImages(view, vaultRef.current, files);
              return true;
            },
            mousedown: (event, view) => {
              if (!(event.ctrlKey || event.metaKey) || !followRef.current) return false;
              const pos = view.posAtCoords({ x: event.clientX, y: event.clientY });
              if (pos === null) return false;
              const target = wikilinkAt(view.state.doc.toString(), pos);
              if (!target) return false;
              event.preventDefault();
              followRef.current(target);
              return true;
            },
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

/** Uploads each image and inserts its markdown at the current selection. */
async function insertImages(view: EditorView, vaultId: string, files: File[]): Promise<void> {
  const snippets: string[] = [];
  for (const file of files) {
    try {
      const uploaded = await uploadImage(vaultId, file);
      snippets.push(imageMarkdown(uploaded.path));
    } catch (error) {
      // No broken link is inserted, and the user hears about the failure.
      useAppStore
        .getState()
        .pushNotice(`图片上传失败：${error instanceof Error ? error.message : String(error)}`, "warn");
    }
  }
  if (snippets.length === 0) return;
  const at = view.state.selection.main.to;
  view.dispatch({ changes: { from: at, insert: snippets.join("\n") } });
}
