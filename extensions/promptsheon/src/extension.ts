'use client';
/**
 * promptsheon VS Code extension — manifest authoring.
 *
 * Three surfaces:
 *
 *  1. Validate-on-save: when a `.promptsheon.json|yaml|yml` file
 *     is saved, POST it to the promptsheon server's
 *     /api/manifests/validate endpoint and surface the issues
 *     as diagnostics on the file.
 *
 *  2. Hover docs: hovering over a node-type identifier (Planner,
 *     Agent, Tool, Guardrail) shows the node's purpose + the
 *     shape of the object it expects.
 *
 *  3. "Send to Playground" command: opens the playground with the
 *     current manifest pre-loaded. The command takes the
 *     playground's URL from `promptsheon.playgroundUrl` in the
 *     user/workspace settings; missing config prompts the user
 *     for the URL on first use.
 */
import * as vscode from 'vscode';
import {
  buildValidateRequestBody,
  isManifestFilename,
  NODE_HOVERS,
  parseValidateResponse,
  validateManifestLocal,
  type DiagnosticLite,
} from './validate.js';

function basename(path: string): string {
  return path.split('/').pop() ?? '';
}

function isManifestDocument(doc: vscode.TextDocument): boolean {
  return isManifestFilename(basename(doc.fileName));
}

function liteToDiagnostic(d: DiagnosticLite, line = 0): vscode.Diagnostic {
  return {
    range: new vscode.Range(line, 0, line, 0),
    severity: vscode.DiagnosticSeverity.Error,
    message: d.message,
    source: 'promptsheon',
  };
}

export function activate(context: vscode.ExtensionContext): void {
  const collection = vscode.languages.createDiagnosticCollection('promptsheon');

  async function validate(doc: vscode.TextDocument): Promise<void> {
    if (!isManifestDocument(doc)) {
      collection.set(doc.uri, []);
      return;
    }
    const text = doc.getText();
    let parsed: unknown;
    try {
      parsed = JSON.parse(text);
    } catch {
      collection.set(doc.uri, [
        {
          range: new vscode.Range(0, 0, 0, 0),
          severity: vscode.DiagnosticSeverity.Error,
          message: 'invalid JSON (the extension does not parse YAML in v1; export to JSON first)',
        },
      ]);
      return;
    }
    const issues = validateManifestLocal(parsed);
    if (issues.length > 0) {
      collection.set(doc.uri, issues.map((i) => liteToDiagnostic(i)));
      return;
    }
    // Local validation passed — ask the server for the canonical
    // verdict. The server's Zod schema is the source of truth.
    const serverIssues = await runServerValidation(parsed);
    collection.set(
      doc.uri,
      serverIssues.map((i) => liteToDiagnostic(i)),
    );
  }

  context.subscriptions.push(
    collection,
    vscode.workspace.onDidSaveTextDocument((doc) => {
      void validate(doc);
    }),
    vscode.workspace.onDidOpenTextDocument((doc) => {
      void validate(doc);
    }),
    vscode.commands.registerCommand('promptsheon.validate', () => {
      const editor = vscode.window.activeTextEditor;
      if (!editor) {
        void vscode.window.showInformationMessage('promptsheon: open a manifest first.');
        return;
      }
      void validate(editor.document);
    }),
    vscode.languages.registerHoverProvider(
      [
        { language: 'json', pattern: '**/.promptsheon.json' },
        { language: 'json', pattern: '**/.promptsheon.yaml' },
      ],
      new PromptsheonHoverProvider(),
    ),
    vscode.commands.registerCommand('promptsheon.sendToPlayground', async () => {
      const editor = vscode.window.activeTextEditor;
      if (!editor || !isManifestDocument(editor.document)) {
        void vscode.window.showErrorMessage('promptsheon: open a .promptsheon.{json,yaml,yml} file first.');
        return;
      }
      const cfg = vscode.workspace.getConfiguration('promptsheon');
      const baseUrl = cfg.get<string>('playgroundUrl') ?? 'http://127.0.0.1:8080';
      const encoded = encodeURIComponent(editor.document.getText());
      const url = `${baseUrl.replace(/\/$/, '')}/app/playground?manifest=${encoded}`;
      await vscode.env.openExternal(vscode.Uri.parse(url));
    }),
  );

  for (const doc of vscode.workspace.textDocuments) {
    void validate(doc);
  }
}

async function runServerValidation(parsed: unknown): Promise<DiagnosticLite[]> {
  const cfg = vscode.workspace.getConfiguration('promptsheon');
  const baseUrl = cfg.get<string>('apiUrl') ?? 'http://127.0.0.1:8080';
  const apiKey = cfg.get<string>('apiKey') ?? '';
  const requestBody = buildValidateRequestBody(parsed);
  try {
    const res = await fetch(`${baseUrl.replace(/\/$/, '')}/api/manifests/validate`, {
      method: 'POST',
      headers: {
        'content-type': 'application/json',
        ...(apiKey ? { authorization: `Bearer ${apiKey}` } : {}),
      },
      body: JSON.stringify(requestBody),
    });
    const json = (await res.json()) as { valid: boolean; issues: DiagnosticLite[] };
    return parseValidateResponse(json);
  } catch {
    // Server offline — local validation already covered the basics;
    // don't block the editor.
    return [];
  }
}

class PromptsheonHoverProvider implements vscode.HoverProvider {
  provideHover(doc: vscode.TextDocument, position: vscode.Position): vscode.ProviderResult<vscode.Hover> {
    if (!isManifestDocument(doc)) return undefined;
    const range = doc.getWordRangeAtPosition(position, /[A-Za-z]+/);
    if (!range) return undefined;
    const word = doc.getText(range);
    const hover = NODE_HOVERS[word];
    if (!hover) return undefined;
    const md = new vscode.MarkdownString();
    md.appendMarkdown(`**${word}** — ${hover.summary}\n\n`);
    md.appendCodeblock(hover.shape, 'json');
    return new vscode.Hover(md, range);
  }
}

export function deactivate(): void {
  // The diagnostic collection is automatically disposed by the
  // extension host; nothing else to clean up.
}