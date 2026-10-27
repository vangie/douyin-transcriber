import { useEffect, useRef, useState } from "react";
import {
  Action,
  ActionPanel,
  Alert,
  Detail,
  Clipboard,
  Form,
  List,
  Icon,
  confirmAlert,
  Toast,
  showToast,
} from "@raycast/api";
import { Catalog, CatalogModel, HistoryItem, forgetHistory, listHistory, listModels, runDyt } from "./dyt";

interface FormValues {
  input: string;
  speech: string;
  chat: string;
  polish: boolean;
}

const AUTO = "";

export default function Command() {
  const [catalog, setCatalog] = useState<Catalog | null>(null);
  const [history, setHistory] = useState<HistoryItem[]>([]);
  const [opened, setOpened] = useState<HistoryItem | null>(null);
  const [composing, setComposing] = useState(false);
  const [clipboard, setClipboard] = useState("");
  const [running, setRunning] = useState(false);
  const [log, setLog] = useState("");
  const [result, setResult] = useState<HistoryItem | null>(null);
  const [failed, setFailed] = useState(false);
  const cancelRef = useRef<(() => void) | null>(null);

  useEffect(() => {
    let active = true;
    listModels()
      .then((found) => {
        if (active) setCatalog(found);
      })
      .catch(async (error: Error) => {
        if (!active) return;
        setCatalog({ speech: [], chat: [] });
        await showToast({ style: Toast.Style.Failure, title: "Could not list models", message: error.message });
      });
    listHistory()
      .then((items) => {
        if (active) setHistory(items);
      })
      .catch(() => undefined);
    Clipboard.readText()
      .then((text) => {
        if (active && text && looksLikeDouyin(text)) setClipboard(text.trim());
      })
      .catch(() => undefined);
    return () => {
      active = false;
      cancelRef.current?.();
    };
  }, []);

  async function handleSubmit(values: FormValues) {
    const input = values.input.trim();
    if (!input) {
      await showToast({ style: Toast.Style.Failure, title: "Paste a Douyin share" });
      return;
    }
    setRunning(true);
    setFailed(false);
    setResult(null);
    setLog("");
    const run = runDyt({
      input,
      speech: values.speech,
      chat: values.chat,
      polish: values.polish,
      onOutput: (chunk) => setLog((current) => current + chunk),
    });
    cancelRef.current = run.cancel;
    try {
      await run.done;
      const items = await listHistory();
      setHistory(items);
      setResult(items[0] ?? null);
    } catch (error) {
      setFailed(true);
      const message = error instanceof Error ? error.message : "Transcription failed";
      await showToast({ style: Toast.Style.Failure, title: "Transcription failed", message });
    } finally {
      cancelRef.current = null;
      setRunning(false);
    }
  }

  if (opened && !running && !log) {
    return (
      <Detail
        markdown={opened.text}
        navigationTitle={opened.title || opened.id}
        actions={
          <ActionPanel>
            <Action.CopyToClipboard title="Copy" content={opened.text} />
            <Action.Paste title="Paste" content={opened.text} />
            <Action title="Back" icon={Icon.ArrowLeft} onAction={() => setOpened(null)} />
            <Action
              title="Delete"
              icon={Icon.Trash}
              style={Action.Style.Destructive}
              shortcut={{ modifiers: ["cmd"], key: "backspace" }}
              onAction={() => removeHistory(opened, setHistory, setOpened)}
            />
          </ActionPanel>
        }
      />
    );
  }

  if (running || log) {
    return (
      <Detail
        isLoading={running}
        markdown={running ? `\`\`\`\n${log || "Starting…"}\n\`\`\`` : transcriptMarkdown(log)}
        actions={
          <ActionPanel>
            {running ? (
              <Action
                title="Stop"
                icon={Icon.Stop}
                onAction={() => cancelRef.current?.()}
              />
            ) : (
              <>
                <Action.CopyToClipboard title="Copy" content={transcriptText(log)} />
                <Action.Paste title="Paste" content={transcriptText(log)} />
                <Action
                  title="New"
                  icon={Icon.ArrowLeft}
                  onAction={() => {
                    setLog("");
                    setResult(null);
                    setFailed(false);
                  }}
                />
                {result && (
                  <Action
                    title="Delete"
                    icon={Icon.Trash}
                    style={Action.Style.Destructive}
                    shortcut={{ modifiers: ["cmd"], key: "backspace" }}
                    onAction={() =>
                      removeHistory(result, setHistory, (item) => {
                        if (item === null) {
                          setResult(null);
                          setLog("");
                        }
                      })
                    }
                  />
                )}
              </>
            )}
          </ActionPanel>
        }
        metadata={
          failed ? (
            <Detail.Metadata>
              <Detail.Metadata.Label title="Status" text="Failed" />
            </Detail.Metadata>
          ) : null
        }
      />
    );
  }

  if (!composing) {
    return (
      <List searchBarPlaceholder="Search transcripts" isLoading={catalog === null && history.length === 0}>
        <List.Section title="New">
          <List.Item
            icon={Icon.Plus}
            title={clipboard ? "Transcribe the link on the clipboard" : "Transcribe a Douyin link"}
            subtitle={clipboard ? clipPreview(clipboard) : "Paste a share link"}
            actions={
              <ActionPanel>
                <Action title="Transcribe" icon={Icon.Microphone} onAction={() => setComposing(true)} />
              </ActionPanel>
            }
          />
        </List.Section>
        <List.Section title="Last 24 hours" subtitle="/tmp/dyt">
          {history.map((item) => (
            <List.Item
              key={item.id}
              icon={Icon.Document}
              title={item.title || item.id}
              accessories={[{ text: when(item.at) }]}
              actions={
                <ActionPanel>
                  <Action title="Open" icon={Icon.Document} onAction={() => setOpened(item)} />
                  <Action.CopyToClipboard title="Copy" content={item.text} />
                  <Action
                    title="Delete"
                    icon={Icon.Trash}
                    style={Action.Style.Destructive}
                    shortcut={{ modifiers: ["cmd"], key: "backspace" }}
                    onAction={() => removeHistory(item, setHistory, setOpened)}
                  />
                  <Action title="New" icon={Icon.Plus} onAction={() => setComposing(true)} />
                </ActionPanel>
              }
            />
          ))}
        </List.Section>
      </List>
    );
  }

  return (
    <Form
      isLoading={catalog === null}
      actions={
        <ActionPanel>
          <Action.SubmitForm title="Transcribe" onSubmit={handleSubmit} />
          {history.length > 0 && <Action title="Recent" icon={Icon.Clock} onAction={() => setComposing(false)} />}
        </ActionPanel>
      }
    >
      <Form.TextArea
        id="input"
        title="Share"
        placeholder="Paste a Douyin link or the whole share text"
        defaultValue={clipboard}
        autoFocus
      />
      <Form.Dropdown id="speech" title="Speech" storeValue>
        <Form.Dropdown.Item value={AUTO} title="Largest local model" />
        {(catalog?.speech ?? []).map((model) => (
          <Form.Dropdown.Item key={model.ref} value={model.ref} title={speechTitle(model)} />
        ))}
      </Form.Dropdown>
      <Form.Checkbox id="polish" label="Rewrite the transcript" defaultValue={true} storeValue />
      <Form.Dropdown id="chat" title="Rewrite" storeValue>
        <Form.Dropdown.Item value={AUTO} title="Magpie, then Ollama" />
        <Form.Dropdown.Item value="ollama-first" title="Ollama, then Magpie" />
        {(catalog?.chat ?? []).map((model) => (
          <Form.Dropdown.Item key={`${model.where}:${model.ref}`} value={model.ref} title={chatTitle(model)} />
        ))}
      </Form.Dropdown>
    </Form>
  );
}

async function removeHistory(
  item: HistoryItem,
  setHistory: (items: HistoryItem[]) => void,
  setOpened: (item: HistoryItem | null) => void,
) {
  const confirmed = await confirmAlert({
    title: "Delete this transcript?",
    message: item.title || item.id,
    primaryAction: { title: "Delete", style: Alert.ActionStyle.Destructive },
  });
  if (!confirmed) return;
  await forgetHistory(item.id);
  setOpened(null);
  setHistory(await listHistory());
}

function clipPreview(text: string): string {
  const line = text.replace(/\s+/g, " ");
  return line.length > 48 ? `${line.slice(0, 48)}…` : line;
}

function when(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "";
  return date.toLocaleString();
}

function looksLikeDouyin(text: string): boolean {
  return /douyin\.com|v\.douyin\.com/.test(text);
}

function speechTitle(model: CatalogModel): string {
  return model.path ? `${model.ref}` : model.ref;
}

function chatTitle(model: CatalogModel): string {
  return `${model.ref} · ${model.where}`;
}

function transcriptText(log: string): string {
  const lines = log.split("\n").filter((line) => !isStatus(line));
  return lines.join("\n").trim();
}

function transcriptMarkdown(log: string): string {
  const text = transcriptText(log);
  return text || "No transcript.";
}

function isStatus(line: string): boolean {
  return /^(warning:|Polished with|Resolving URL|Resolved:|Downloading|Downloaded|Extracting|Audio downloaded|Transcribing|Speech:|Polish:|Polishing|Video:|Using installed|whisper_|main:|ggml_|system_info:|load_backend:|read_audio_data:)/.test(
    line.trim(),
  );
}
