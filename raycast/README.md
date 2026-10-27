# Douyin Transcript

A local Raycast extension. It calls the `dyt` command already installed on this Mac.

Open Raycast, search for **Douyin Transcript**, paste a share link, and pick the speech model and the rewrite model. Leave both on automatic to use the largest local ggml file and Magpie, then Ollama.

## Develop

```bash
npm install
npm run dev
```

`npm run dev` imports the extension into Raycast. The command stays available after the process exits.

Set **dyt binary** in the command preferences when `dyt` is not in `~/.local/bin` or Homebrew.
