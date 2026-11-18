import { TransportError } from "./transport";

type UploadResult = { path: string; existed: boolean };

/** Uploads one image through the kbimg protocol (D-009: raw bytes, no base64). */
export async function uploadImage(vaultId: string, blob: Blob): Promise<UploadResult> {
  const body = await blob.arrayBuffer();
  const response = await fetch(`http://kbimg.localhost/__upload?vault=${encodeURIComponent(vaultId)}`, {
    method: "POST",
    body,
  });
  const payload = (await response.json()) as { path?: string; existed?: boolean; failure?: { code: string; message: string } };
  if (payload.failure) throw new TransportError(payload.failure.code, payload.failure.message);
  if (!payload.path) throw new TransportError("unknown", "上传没有返回路径");
  return { path: payload.path, existed: Boolean(payload.existed) };
}

/** The markdown inserted for an uploaded image. */
export function imageMarkdown(path: string): string {
  return `![](${path})`;
}
