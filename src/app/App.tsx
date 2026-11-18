import { useEffect, useState } from "react";
import { ping } from "../lib/transport";

export function App() {
  const [status, setStatus] = useState("连接中");

  useEffect(() => {
    let alive = true;
    ping()
      .then((value) => {
        if (alive) setStatus(`Go 后端: ${value}`);
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
