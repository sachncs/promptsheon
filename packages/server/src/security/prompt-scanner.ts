import type { Finding, FindingSeverity, PromptVerdict } from '../repos/prompt-scan.js';

/**
 * PromptSecurityScanner — T2-3 static analyzer that classifies
 * user-authored content before save. Three families of rules:
 *
 *   1. PII detection: email, US SSN, credit-card (Luhn), phone
 *      numbers, US/EU IBAN, IP address. Runs as regex matchers
 *      against the literal text and never relies on an LLM call.
 *
 *   2. Shell-injection / prompt-injection heuristics: patterns
 *      that try to break out of a prompt-and-response contract
 *      (ignore previous instructions, system: overrides, etc).
 *
 *   3. Jailbreak patterns: well-known attack phrases from the
 *      OWASP LLM01..LLM10 threat catalogue. We pin the canonical
 *      phrase so a future LLM tweak doesn't drift.
 *
 * The scanner is heuristic — it errs on the side of `warn` when
 * ambiguous. Customers can override per-rule via `promptsheon.scanner.rules.<rule>`
 * in the Settings store (T2-4 surface).
 *
 * Final verdict:
 *   - block: any 'block' finding
 *   - warn:  any 'warn' finding, no 'block'
 *   - clean: no findings
 */

interface ScannerInput {
  text: string;
  /** Optional pre-existing findings (caller can skip a family). */
  skip?: Array<'pii' | 'injection' | 'jailbreak'>;
}

interface RuleHit {
  rule: string;
  severity: FindingSeverity;
  description: string;
  re: RegExp;
}

const PII_RULES: RuleHit[] = [
  {
    rule: 'pii.email',
    severity: 'warn',
    description: 'Email address detected',
    re: /[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}/g,
  },
  {
    rule: 'pii.ssn',
    severity: 'block',
    description: 'US SSN detected',
    re: /\b\d{3}-\d{2}-\d{4}\b/g,
  },
  {
    rule: 'pii.credit-card',
    severity: 'block',
    description: 'Possible credit-card number detected',
    re: /\b(?:\d[ -]?){13,16}\d\b/g,
  },
  {
    rule: 'pii.phone',
    severity: 'warn',
    description: 'Phone number detected',
    re: /\b(?:\+?\d{1,3}[ -]?)?\(?\d{3}\)?[ -]?\d{3}[ -]?\d{4}\b/g,
  },
  {
    rule: 'pii.iban',
    severity: 'block',
    description: 'IBAN detected',
    re: /\b[A-Z]{2}\d{2}[A-Z0-9]{11,30}\b/g,
  },
  {
    rule: 'pii.ipv4',
    severity: 'warn',
    description: 'IPv4 address detected',
    re: /\b(?:\d{1,3}\.){3}\d{1,3}\b/g,
  },
  {
    rule: 'pii.aws-key',
    severity: 'block',
    description: 'Possible AWS access key id detected',
    re: /\bAKIA[0-9A-Z]{16}\b/g,
  },
  {
    rule: 'pii.private-key',
    severity: 'block',
    description: 'PEM private key header detected',
    re: /-----BEGIN (?:RSA |EC |DSA |OPENSSH |PGP )?PRIVATE KEY-----/g,
  },
];

const INJECTION_RULES: RuleHit[] = [
  {
    rule: 'injection.ignore-previous',
    severity: 'block',
    description: 'Prompt-injection: "ignore previous instructions" pattern',
    re: /\b(?:ignore|disregard|forget|skip)\b[^.\n]*\b(?:previous|prior|above|earlier)\b[^.\n]*\b(?:instructions?|rules?|prompts?|directives?)\b/gi,
  },
  {
    rule: 'injection.system-override',
    severity: 'block',
    description: 'Prompt-injection: "system:" override pattern',
    re: /^\s*(?:system|assistant)\s*:\s*/gim,
  },
  {
    rule: 'injection.role-switch',
    severity: 'block',
    description: 'Prompt-injection: explicit role-switch instruction',
    re: /\b(?:you are now|act as|pretend to be|roleplay as|simulate being)\b[^.\n]{0,80}\b(?:developer|admin|root|jailbreak)\b/gi,
  },
  {
    rule: 'injection.instruction-injection',
    severity: 'block',
    description: 'Prompt-injection: instruction-bypass attempt',
    re: /\b(?:reveal|print|leak|exfiltrate|disclose)\b[^.\n]{0,80}\b(?:system\s*prompt|hidden\s*instructions?|developer\s*message|secret)\b/gi,
  },
  {
    rule: 'injection.tool-abuse',
    severity: 'warn',
    description: 'Suspicious tool-call instruction (potential data exfiltration)',
    re: /\b(?:curl|wget|fetch|exec|eval)\s*\(?[^.\n]{0,80}\b(?:https?:\/\/|env|secret|token)\b/gi,
  },
];

const JAILBREAK_RULES: RuleHit[] = [
  {
    rule: 'jailbreak.dan',
    severity: 'block',
    description: 'Jailbreak: "do-anything-now" pattern',
    re: /\b(?:do anything now|DAN|jailbreak)\b/gi,
  },
  {
    rule: 'jailbreak.token-leak',
    severity: 'block',
    description: 'Jailbreak: attempt to leak hidden prompt',
    re: /\b(?:print|repeat|reveal)\b[^.\n]{0,80}\b(?:above|previous|hidden)\b[^.\n]{0,80}\b(?:prompt|instructions?|system)\b/gi,
  },
  {
    rule: 'jailbreak.evil-twin',
    severity: 'warn',
    description: 'Jailbreak: "DAN-style" persona request',
    re: /\b(?:you are (?:now )?a|act as|roleplay|impersonate)\b[^.\n]{0,80}\b(?:jailbroken|unethical|unfiltered)\b/gi,
  },
  {
    rule: 'jailbreak.reverse-shell',
    severity: 'block',
    description: 'Jailbreak: payload exfiltration / reverse shell pattern',
    re: /\b(?:curl|wget|fetch)\b[^.\n]{0,80}\b(?:attacker|evil|c2|webhook|burpcollaborator|interactsh)\b/gi,
  },
];

const ALL_RULES: RuleHit[] = [...PII_RULES, ...INJECTION_RULES, ...JAILBREAK_RULES];

const RULE_BY_NAME: Map<string, RuleHit> = new Map(ALL_RULES.map((r) => [r.rule, r]));

/**
 * Luhn check for credit-card numbers. The regex above is a
 * permissive shape match; this prunes false positives before
 * reporting.
 */
function luhnValid(s: string): boolean {
  const digits = s.replace(/\D/g, '');
  if (digits.length < 13 || digits.length > 19) return false;
  let sum = 0;
  let alt = false;
  for (let i = digits.length - 1; i >= 0; i -= 1) {
    let n = digits.charCodeAt(i) - 48;
    if (n < 0 || n > 9) return false;
    if (alt) {
      n *= 2;
      if (n > 9) n -= 9;
    }
    sum += n;
    alt = !alt;
  }
  return sum % 10 === 0;
}

/**
 * Scan text against all enabled rules. Returns the findings +
 * the final verdict. A caller can disable rules by passing a
 * settings override map.
 */
export function scan(input: ScannerInput, disabled?: Set<string>): {
  verdict: PromptVerdict;
  findings: Finding[];
} {
  if (!input.text) return { verdict: 'clean', findings: [] };
  const skip = new Set(input.skip ?? []);
  const findings: Finding[] = [];

  const families: Record<'pii' | 'injection' | 'jailbreak', RuleHit[]> = {
    pii: PII_RULES,
    injection: INJECTION_RULES,
    jailbreak: JAILBREAK_RULES,
  };
  const familyFor = (rule: string): 'pii' | 'injection' | 'jailbreak' | null => {
    if (rule.startsWith('pii.')) return 'pii';
    if (rule.startsWith('injection.')) return 'injection';
    if (rule.startsWith('jailbreak.')) return 'jailbreak';
    return null;
  };

  for (const rule of ALL_RULES) {
    if (disabled?.has(rule.rule)) continue;
    const family = familyFor(rule.rule);
    if (family && skip.has(family)) continue;
    rule.re.lastIndex = 0;
    let m: RegExpExecArray | null;
    while ((m = rule.re.exec(input.text)) !== null) {
      if (rule.rule === 'pii.credit-card' && !luhnValid(m[0])) continue;
      findings.push({
        rule: rule.rule,
        severity: rule.severity,
        message: rule.description,
        range: { start: m.index, end: m.index + m[0].length },
        snippet: m[0].length > 64 ? m[0].slice(0, 64) + '…' : m[0],
      });
      if (!rule.re.global) break;
    }
  }

  let verdict: PromptVerdict = 'clean';
  if (findings.some((f) => f.severity === 'block')) verdict = 'block';
  else if (findings.some((f) => f.severity === 'warn')) verdict = 'warn';

  return { verdict, findings };
}

/**
 * Lint helper exposed for tests so callers can inspect the
 * available rules without re-walking the rules array.
 */
export function listRules(): Array<{ rule: string; severity: FindingSeverity; description: string }> {
  return ALL_RULES.map((r) => ({ rule: r.rule, severity: r.severity, description: r.description }));
}

export { RULE_BY_NAME };
