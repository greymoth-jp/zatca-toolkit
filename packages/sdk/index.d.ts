// Type definitions for @zatca/sdk. The runtime is ESM JavaScript (src/*.mjs); these types
// make it consumable from TypeScript. (A full .ts source migration is mechanical follow-up.)

export interface RuleError {
  rule_id: string;
  path: string;
  message_en: string;
  message_ar: string;
  severity: 'fatal' | 'warning';
}

export interface ValidationReport {
  ok: boolean;
  error?: string;
  report?: { valid: boolean; errors: RuleError[] };
}

export interface GenerateResult {
  ok: boolean;
  error?: string;
  ubl?: string;
  cii?: string;
}

export interface FacturXResult {
  ok: boolean;
  error?: string;
  /** Base64-encoded PDF/A-3 (Factur-X) bytes. */
  pdf?: string;
  /** Decoded PDF bytes (convenience; present when ok). */
  pdfBytes?: Uint8Array;
}

export interface QRResult {
  ok: boolean;
  error?: string;
  findings?: RuleError[];
}

export interface Engine {
  validateXML(xml: string, profile?: 'peppol-bis' | 'en16931' | 'zatca-ksa'): ValidationReport;
  validateDoc(doc: unknown, profile?: 'peppol-bis' | 'en16931' | 'zatca-ksa'): ValidationReport;
  generateUBL(doc: unknown): GenerateResult;
  generateCII(doc: unknown): GenerateResult;
  /** Render a Factur-X PDF/A-3 (CII embedded). PDF/A-3 structure; not veraPDF-certified. */
  generateFacturX(doc: unknown): FacturXResult;
  validateQR(qr: string, opts?: { signed?: boolean; simplified?: boolean }): QRResult;
  validateStructure(xml: string): QRResult;
  version(): { ok: boolean; version?: string };
}

export function loadEngine(): Promise<Engine>;

export interface ClearResult {
  status: string;
  uuid: string;
  mock?: boolean;
  note?: string;
  idempotentReplay?: boolean;
}

export interface ZatcaOptions {
  apiKey?: string;
  mode?: 'sandbox' | 'production';
  clearer?: (signedXml: string, o?: { idempotencyKey?: string }) => Promise<ClearResult>;
}

export class Zatca {
  constructor(opts?: ZatcaOptions);
  /** Validate a UBL/ZATCA XML string. Defaults to 'zatca-ksa' (KSA-first toolkit). */
  validateXML(xml: string, profile?: 'zatca-ksa' | 'en16931' | 'peppol-bis'): Promise<ValidationReport>;
  /** Validate a normalized invoice object. Defaults to 'zatca-ksa'. */
  validate(doc: unknown, profile?: 'zatca-ksa' | 'en16931' | 'peppol-bis'): Promise<ValidationReport>;
  generate(doc: unknown): Promise<GenerateResult>;
  /** Render a Factur-X PDF/A-3 (CII embedded). PDF/A-3 structure; not veraPDF-certified. */
  generateFacturX(doc: unknown): Promise<FacturXResult>;
  clear(signedXml: string, o?: { idempotencyKey?: string }): Promise<ClearResult>;
}

export function verifyWebhook(rawBody: string | Buffer, signatureHeader: string, secret: string): boolean;
export function signWebhook(rawBody: string | Buffer, secret: string): string;
