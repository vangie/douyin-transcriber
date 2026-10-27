import { execFile } from "child_process";
import { promisify } from "util";
import { getPreferenceValues } from "@raycast/api";
import fs from "fs";
import os from "os";
import path from "path";

const execFileAsync = promisify(execFile);

export interface CatalogModel {
  ref: string;
  where: string;
  path?: string;
}

export interface Catalog {
  speech: CatalogModel[];
  chat: CatalogModel[];
}

interface Preferences {
  dytPath?: string;
}

export function dytBinary(): string {
  const configured = getPreferenceValues<Preferences>().dytPath?.trim();
  if (configured) return configured;
  const candidates = [
    path.join(os.homedir(), ".local", "bin", "dyt"),
    "/opt/homebrew/bin/dyt",
    "/usr/local/bin/dyt",
  ];
  return candidates.find((candidate) => fs.existsSync(candidate)) ?? "dyt";
}

export async function listModels(): Promise<Catalog> {
  const { stdout, stderr } = await execFileAsync(dytBinary(), ["--list"], {
    timeout: 15000,
    env: process.env,
  });
  // dyt prints the catalog on stderr.
  return parseList(`${stdout}\n${stderr}`);
}

export function parseList(text: string): Catalog {
  const catalog: Catalog = { speech: [], chat: [] };
  let section: "speech" | "chat" | null = null;
  for (const line of text.split("\n")) {
    const trimmed = line.trim();
    if (trimmed.startsWith("Speech models")) {
      section = "speech";
      continue;
    }
    if (trimmed.startsWith("Chat models")) {
      section = "chat";
      continue;
    }
    if (!section || (!trimmed.startsWith("speech ") && !trimmed.startsWith("chat "))) continue;
    const file = trimmed.match(/^(speech|chat)\s+(.+)\s+\((.+)\)$/);
    const served = trimmed.match(/^(speech|chat)\s+(.+)\s+@\s+(\S+)$/);
    if (file) {
      catalog[file[1] as "speech" | "chat"].push({ ref: file[2], where: "whisper.cpp", path: file[3] });
      continue;
    }
    if (served) {
      catalog[served[1] as "speech" | "chat"].push({ ref: served[2], where: served[3] });
    }
  }
  return catalog;
}

export interface HistoryItem {
  id: string;
  title: string;
  url: string;
  input: string;
  text: string;
  speech: string;
  chat: string;
  at: string;
}

export async function forgetHistory(id: string): Promise<void> {
  await execFileAsync(dytBinary(), ["--forget", id], { timeout: 10000, env: process.env });
}

export async function listHistory(): Promise<HistoryItem[]> {
  const { stdout } = await execFileAsync(dytBinary(), ["--history"], {
    timeout: 10000,
    env: process.env,
  });
  const trimmed = stdout.trim();
  if (!trimmed) return [];
  return JSON.parse(trimmed) as HistoryItem[];
}

export interface RunRequest {
  input: string;
  speech: string;
  chat: string;
  polish: boolean;
  onOutput: (chunk: string) => void;
}

export function runDyt(request: RunRequest): { cancel: () => void; done: Promise<void> } {
  const args = ["-l", "zh"];
  if (request.speech) args.push("--speech", request.speech);
  if (!request.polish) args.push("--no-polish");
  else if (request.chat) args.push("--chat", request.chat);
  args.push(request.input);

  const child = execFile(dytBinary(), args, { env: process.env });
  child.stdout?.on("data", (chunk: Buffer) => request.onOutput(chunk.toString()));
  child.stderr?.on("data", (chunk: Buffer) => request.onOutput(chunk.toString()));

  const done = new Promise<void>((resolve, reject) => {
    child.on("error", reject);
    child.on("close", (code) => {
      if (code === 0) resolve();
      else reject(new Error(`dyt exited with code ${code ?? "unknown"}`));
    });
  });

  return {
    cancel: () => child.kill("SIGTERM"),
    done,
  };
}
