import { createRoot } from "react-dom/client";
import { App } from "./app/App";
import "./styles/main.css";

const root = document.getElementById("root");
if (!root) throw new Error("#root is missing in index.html");

createRoot(root).render(<App />);
