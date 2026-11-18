import { useEffect, useState } from "react";
import { windowApi } from "../lib/transport";

export function App() {
  const [status, setStatus] = useState("正在连接 Go 后端");

  useEffect(() => {
    let alive = true;
    windowApi
      .ready()
      .then(() => {
        if (alive) setStatus("Go 后端已就绪");
      })
      .catch((error: unknown) => {
        if (alive) setStatus(`Go 后端不可用: ${String(error)}`);
      });
    return () => {
      alive = false;
    };
  }, []);

  return (
    <div className="boot">
      <h1>Mile Knowledge Base</h1>
      <p className="muted">{status}</p>
    </div>
  );
}
