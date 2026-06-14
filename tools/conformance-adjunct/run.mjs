#!/usr/bin/env node
// run.mjs — OPT-IN official-Schematron conformance for ZATCA Toolkit.
//
// This is NOT part of the core library or the WASM/browser engine. It is a separate, opt-in
// adjunct for users who want CERTIFIED-GRADE parity by running the OFFICIAL rules (EN16931 ~200,
// Peppol BIS ~130) instead of the curated subset the zero-dependency engine ships. It drives
// Saxon-HE over the official Schematron and reads the SVRL result.
//
// Profiles:
//   --profile en16931  (default)  official EN16931 UBL validation XSLT (ready-to-run, EUPL-1.2)
//   --profile peppol              official PEPPOL-EN16931-UBL Schematron, compiled at runtime
//                                 via the ISO Schematron skeleton (self-contained .sch).
//
// Licensing / redistribution (LEGAL_RISK):
//   - Saxon-HE 10.9 = Mozilla Public License 2.0 (permissive). Downloaded at runtime, not vendored.
//   - Official EN16931 XSLT = EUPL-1.2. Fetched at runtime, NEVER committed/redistributed.
//   - PEPPOL-EN16931-UBL.sch (OpenPEPPOL) + the ISO Schematron skeleton: fetched at runtime,
//     not vendored. (Verify upstream licence before any redistribution; we do not redistribute.)
//   - Requires a Java runtime (JRE 8+). With no Java the tool exits BLOCKED; it never fabricates.
//   - NOT tax advice and NOT a compliance guarantee.
//
// Usage:
//   node tools/conformance-adjunct/run.mjs <invoice.xml> [--profile en16931|peppol]
//   ZATCA_CONFORMANCE_CACHE=/dir node tools/conformance-adjunct/run.mjs <invoice.xml> --profile peppol
// Exit codes: 0 = no failed assertions, 1 = failed assertions, 2 = BLOCKED (no Java), 3 = error.

import { spawnSync } from 'node:child_process';
import { mkdirSync, existsSync, readFileSync, createWriteStream } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { get } from 'node:https';

const SAXON_URL = 'https://repo1.maven.org/maven2/net/sf/saxon/Saxon-HE/10.9/Saxon-HE-10.9.jar';
const EN16931_XSLT_URL = 'https://raw.githubusercontent.com/ConnectingEurope/eInvoicing-EN16931/master/ubl/xslt/EN16931-UBL-validation.xslt';
const PEPPOL_SCH_URL = 'https://raw.githubusercontent.com/OpenPEPPOL/peppol-bis-invoice-3/master/rules/sch/PEPPOL-EN16931-UBL.sch';
const ISO_SVRL_URL = 'https://raw.githubusercontent.com/OpenPEPPOL/peppol-bis/master/script/iso-schematron-xslt2/iso_svrl_for_xslt2.xsl';
const ISO_SKELETON_URL = 'https://raw.githubusercontent.com/OpenPEPPOL/peppol-bis/master/script/iso-schematron-xslt2/iso_schematron_skeleton_for_saxon.xsl';

const cacheDir = process.env.ZATCA_CONFORMANCE_CACHE || join(tmpdir(), 'zatca-conformance-cache');
const saxonJar = join(cacheDir, 'Saxon-HE-10.9.jar');

function fail(code, msg) { console.error(msg); process.exit(code); }

function download(url, dest) {
  return new Promise((resolve, reject) => {
    const file = createWriteStream(dest);
    get(url, (res) => {
      if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
        file.close(); return download(res.headers.location, dest).then(resolve, reject);
      }
      if (res.statusCode !== 200) { file.close(); return reject(new Error(`HTTP ${res.statusCode} for ${url}`)); }
      res.pipe(file);
      file.on('finish', () => file.close(resolve));
    }).on('error', reject);
  });
}

async function ensure(url, name) {
  const dest = join(cacheDir, name);
  if (!existsSync(dest)) { console.error(`[adjunct] fetch ${name}`); await download(url, dest); }
  return dest;
}

function haveJava() {
  const r = spawnSync('java', ['-version'], { encoding: 'utf8' });
  return r.status === 0 || /version/i.test(r.stderr || '');
}

function saxon(xsl, src, out) {
  const r = spawnSync('java', ['-jar', saxonJar, `-xsl:${xsl}`, `-s:${src}`, `-o:${out}`], { encoding: 'utf8' });
  if (r.status !== 0 || !existsSync(out)) throw new Error(`Saxon failed: ${r.stderr || r.stdout}`);
}

// Resolve the ready-to-run validation XSLT for the chosen profile, compiling the Schematron if needed.
async function resolveXslt(profile) {
  if (profile === 'en16931') return ensure(EN16931_XSLT_URL, 'EN16931-UBL-validation.xslt');
  if (profile === 'peppol') {
    const compiled = join(cacheDir, 'PEPPOL-EN16931-UBL.compiled.xsl');
    if (!existsSync(compiled)) {
      const sch = await ensure(PEPPOL_SCH_URL, 'PEPPOL-EN16931-UBL.sch');
      await ensure(ISO_SKELETON_URL, 'iso_schematron_skeleton_for_saxon.xsl'); // included by iso_svrl
      const svrl = await ensure(ISO_SVRL_URL, 'iso_svrl_for_xslt2.xsl');
      console.error('[adjunct] compiling Peppol Schematron -> XSLT (one-time)');
      saxon(svrl, sch, compiled);
    }
    return compiled;
  }
  fail(3, `unknown profile: ${profile} (use en16931 or peppol)`);
}

function parseSVRL(svrl) {
  const findings = [];
  const re = /<svrl:failed-assert\b([^>]*)>([\s\S]*?)<\/svrl:failed-assert>/g;
  let m;
  while ((m = re.exec(svrl)) !== null) {
    const loc = (m[1].match(/location="([^"]*)"/) || [])[1] || '';
    const flag = (m[1].match(/flag="([^"]*)"/) || [])[1] || '';
    const text = (m[2].match(/<svrl:text>([\s\S]*?)<\/svrl:text>/) || [])[1] || '';
    findings.push({ flag, location: loc, message: text.replace(/\s+/g, ' ').trim() });
  }
  return { fired: (svrl.match(/<svrl:fired-rule\b/g) || []).length, findings };
}

async function main() {
  const args = process.argv.slice(2);
  const input = args.find((a) => !a.startsWith('--'));
  const pi = args.indexOf('--profile');
  const profile = pi >= 0 ? args[pi + 1] : 'en16931';
  if (!input || !existsSync(input)) fail(3, 'usage: node run.mjs <invoice.xml> [--profile en16931|peppol]  (file not found)');
  if (!haveJava()) fail(2, '[adjunct] BLOCKED: no Java runtime found. Install a JRE 8+ to run the official ' +
    'Schematron. The core @zatca/sdk engine validates without Java (curated subset).');

  mkdirSync(cacheDir, { recursive: true });
  await ensure(SAXON_URL, 'Saxon-HE-10.9.jar');
  const xslt = await resolveXslt(profile);

  const out = join(cacheDir, `result.${profile}.svrl`);
  try { saxon(xslt, input, out); } catch (e) { fail(3, `[adjunct] ${e.message}`); }

  const { fired, findings } = parseSVRL(readFileSync(out, 'utf8'));
  const report = { profile, engine: 'saxon-he-10.9 (official Schematron, runtime-fetched)', firedRules: fired, failedAsserts: findings.length, findings };
  console.log(JSON.stringify(report, null, 2));
  console.error(`[adjunct] profile=${profile}: ${fired} official rules evaluated, ${findings.length} failed. NOT tax advice.`);
  process.exit(findings.length === 0 ? 0 : 1);
}

main().catch((e) => fail(3, `[adjunct] error: ${e.message}`));
