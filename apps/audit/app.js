// app.js — The Clearance Statement. Fully client-side: the invoice is validated in-browser
// by the Go WASM engine and never leaves the device. The verdict is an official-style ink
// STAMP slammed onto a clearance certificate (fear -> relief). Arabic-first, bilingual.

const I18N = {
  en: {
    dir: 'ltr',
    mastheadAr: 'بيان حالة التخليص',
    mastheadEn: 'CLEARANCE STATEMENT · ZATCA TOOLKIT',
    refDoc: 'FORM ZX-1 · SELF-CHECK',
    refWave: 'FATOORA PHASE 2 · WAVE 24',
    headline: 'An invoice that is <span class="fear">not cleared</span> is not paid.',
    lede: 'Submit your invoice for an instant clearance check. It runs entirely in your browser — your invoice never leaves your device.',
    engineLoading: 'loading rule engine…',
    engineReady: 'rule engine ready · deterministic · offline',
    engineFail: 'engine failed to load',
    inputLabel: 'Invoice UBL XML',
    placeholder: '<Invoice xmlns=…>  … paste your ZATCA / UBL 2.1 invoice XML here …',
    sampleGood: 'valid sample',
    sampleBad: 'broken sample',
    clearBtn: 'clear',
    runBtn: 'Stamp it',
    runningBtn: 'stamping…',
    checking: 'parsing UBL · applying EN16931 + ZATCA rules · deterministic, offline…',
    errTitle: 'Could not read that invoice.',
    errHint: 'Expected a UBL 2.1 Invoice (root <Invoice>). Try the sample buttons above.',
    certDoc: 'CLEARANCE STATEMENT',
    clearedStamp: 'CLEARED', clearedStampAr: 'مُخلَّصة',
    failedStamp: 'REJECTED', failedStampAr: 'مرفوضة',
    clearedTitle: 'Main rules passed.',
    clearedSub: 'But ZATCA rules keep changing. The next revision could silently break your invoices — automate keeping up.',
    failedTitle: (n) => `This invoice would NOT clear.`,
    failedSub: 'An uncleared invoice is not a valid tax invoice: your buyer cannot deduct the VAT, and your payment stops.',
    tallyFatalUnit: (n) => `fatal ${n === 1 ? 'finding' : 'findings'}`,
    tallyCleanUnit: 'issues found',
    refFindings: (n) => `${n} FINDING${n === 1 ? '' : 'S'}`,
    shareBtn: 'Share the certificate',
    fixBtn: 'How to fix it',
    downloadBtn: 'download certificate',
    discTag: '// NOT TAX ADVICE',
    disclaimer: ' This tool reports results against published EN16931 / ZATCA rule sets. It is not tax advice and not a compliance guarantee. Live clearance requires your onboarded EGS + ZATCA credentials or a certified partner. Final compliance is your responsibility. Your invoice is processed entirely in your browser and is never uploaded.',
    oss: 'Open source · Apache-2.0 ·',
    shareHeadline: (c) => c ? 'My invoice is ZATCA-ready.' : 'My invoice would NOT clear ZATCA.',
    shareTagline: 'Check yours free — it never leaves your browser.',
    // QR decoder
    qrSectionTitle: 'QR CODE · TLV DECODED',
    qrTagLabels: {
      1: 'Seller Name',
      2: 'VAT Registration No.',
      3: 'Timestamp',
      4: 'Invoice Total (incl. VAT)',
      5: 'VAT Amount',
      6: 'Invoice Hash (SHA-256)',
      7: 'ECDSA Signature',
      8: 'Public Key (ECDSA)',
      9: 'ECDSA OID',
    },
    qrTagLabelsAr: {
      1: 'اسم البائع',
      2: 'الرقم الضريبي',
      3: 'التوقيت',
      4: 'إجمالي الفاتورة (شامل الضريبة)',
      5: 'مبلغ الضريبة',
      6: 'بصمة الفاتورة',
      7: 'التوقيع الرقمي',
      8: 'المفتاح العام',
      9: 'معرّف خوارزمية التوقيع',
    },
    qrNoQr: 'No ZATCA QR found in this invoice.',
    qrInvalid: 'QR data could not be decoded — may be truncated or not TLV-encoded.',
    qrPasteHint: 'Or paste a raw Base64 QR string here to decode it:',
    qrDecodeBtn: 'Decode QR',
    // findings grouping
    findingsSectionFatal: 'FATAL — will block clearance',
    findingsSectionWarning: 'WARNINGS — should be corrected',
    findingFix: 'Fix:',
  },
  ar: {
    dir: 'rtl',
    mastheadAr: 'بيان حالة التخليص',
    mastheadEn: 'مجموعة أدوات زاتكا',
    refDoc: 'نموذج ZX-1 · فحص ذاتي',
    refWave: 'فاتورة المرحلة الثانية · الموجة ٢٤',
    headline: 'الفاتورة التي <span class="fear">لا تُخلَّص</span> لا تُدفع.',
    lede: 'قدّم فاتورتك لفحص فوري للتخليص. يعمل بالكامل داخل متصفحك — ولا تغادر فاتورتك جهازك أبداً.',
    engineLoading: 'جارٍ تحميل محرك القواعد…',
    engineReady: 'محرك القواعد جاهز · حتمي · دون اتصال',
    engineFail: 'فشل تحميل المحرك',
    inputLabel: 'فاتورة UBL XML',
    placeholder: '<Invoice xmlns=…>  … الصق هنا ملف XML لفاتورة زاتكا / UBL 2.1 …',
    sampleGood: 'نموذج صحيح',
    sampleBad: 'نموذج معطوب',
    clearBtn: 'مسح',
    runBtn: 'اختم الفاتورة',
    runningBtn: 'جارٍ الختم…',
    checking: 'تحليل UBL · تطبيق قواعد EN16931 + زاتكا · حتمي، دون اتصال…',
    errTitle: 'تعذّرت قراءة هذه الفاتورة.',
    errHint: 'المتوقع فاتورة UBL 2.1 (الجذر <Invoice>). جرّب أزرار النماذج أعلاه.',
    certDoc: 'بيان حالة التخليص',
    clearedStamp: 'CLEARED', clearedStampAr: 'مُخلَّصة',
    failedStamp: 'REJECTED', failedStampAr: 'مرفوضة',
    clearedTitle: 'اجتازت القواعد الأساسية.',
    clearedSub: 'لكن قواعد زاتكا تتغيّر باستمرار. قد يُبطل التعديل القادم فواتيرك بصمت — اجعل المتابعة تلقائية.',
    failedTitle: (n) => `هذه الفاتورة لن تُخلَّص.`,
    failedSub: 'الفاتورة غير المُخلَّصة ليست فاتورة ضريبية صحيحة: لا يستطيع المشتري خصم الضريبة، ويتوقّف تحصيلك.',
    tallyFatalUnit: (n) => `مخالفة قاتلة`,
    tallyCleanUnit: 'مخالفات',
    refFindings: (n) => `${toAr(n)} مخالفة`,
    shareBtn: 'شارك الشهادة',
    fixBtn: 'كيف تُصلحها',
    downloadBtn: 'تنزيل الشهادة',
    discTag: '// ليست استشارة ضريبية',
    disclaimer: ' تعرض هذه الأداة النتائج وفق مجموعات قواعد EN16931 / زاتكا المنشورة. ليست استشارة ضريبية ولا ضمان امتثال. يتطلّب التخليص الفعلي اعتماد EGS وبيانات اعتماد زاتكا أو شريكاً معتمداً. الامتثال النهائي مسؤوليتك. تُعالَج فاتورتك بالكامل داخل متصفحك ولا تُرفع أبداً.',
    oss: 'مفتوح المصدر · Apache-2.0 ·',
    shareHeadline: (c) => c ? 'فاتورتي جاهزة لزاتكا.' : 'فاتورتي لن تُخلَّص في زاتكا.',
    shareTagline: 'افحص فاتورتك مجاناً — لا تغادر متصفحك.',
    // QR decoder
    qrSectionTitle: 'رمز QR · فك ترميز TLV',
    qrTagLabels: {
      1: 'Seller Name',
      2: 'VAT Registration No.',
      3: 'Timestamp',
      4: 'Invoice Total (incl. VAT)',
      5: 'VAT Amount',
      6: 'Invoice Hash (SHA-256)',
      7: 'ECDSA Signature',
      8: 'Public Key (ECDSA)',
      9: 'ECDSA OID',
    },
    qrTagLabelsAr: {
      1: 'اسم البائع',
      2: 'الرقم الضريبي',
      3: 'التوقيت',
      4: 'إجمالي الفاتورة (شامل الضريبة)',
      5: 'مبلغ الضريبة',
      6: 'بصمة الفاتورة',
      7: 'التوقيع الرقمي',
      8: 'المفتاح العام',
      9: 'معرّف خوارزمية التوقيع',
    },
    qrNoQr: 'لم يُعثر على رمز QR لزاتكا في هذه الفاتورة.',
    qrInvalid: 'تعذّر فك ترميز بيانات QR — ربما تالفة أو غير مرمَّزة بـ TLV.',
    qrPasteHint: 'أو الصق سلسلة Base64 لرمز QR هنا لفك ترميزها:',
    qrDecodeBtn: 'فك ترميز QR',
    // findings grouping
    findingsSectionFatal: 'قاتل — يمنع التخليص',
    findingsSectionWarning: 'تحذيرات — يجب تصحيحها',
    findingFix: 'الإصلاح:',
  },
};

const AR_DIGITS = '٠١٢٣٤٥٦٧٨٩';
function toAr(n) { return String(n).replace(/[0-9]/g, (d) => AR_DIGITS[+d]); }

let lang = 'en';
let engine = null;
let lastResult = null;
const $ = (id) => document.getElementById(id);

// ---- ZATCA QR TLV decoder ----
// ZATCA Phase 2 QR: Base64-encoded TLV stream.
// Each TLV entry: 1 byte tag, 1 byte length, N bytes value (UTF-8 for tags 1-5, bytes for 6-9).
// Tags: 1=seller, 2=VAT reg, 3=timestamp, 4=total+VAT, 5=VAT amount,
//       6=invoice hash, 7=ECDSA sig, 8=public key, 9=ECDSA OID.
function decodeTlv(b64) {
  let bytes;
  try {
    const bin = atob(b64.trim().replace(/\s+/g, ''));
    bytes = new Uint8Array(bin.length);
    for (let i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i);
  } catch (_) {
    return null;
  }
  const tags = {};
  let i = 0;
  const dec = new TextDecoder('utf-8');
  while (i < bytes.length - 1) {
    const tag = bytes[i++];
    if (i >= bytes.length) break;
    const len = bytes[i++];
    if (i + len > bytes.length) break;
    const val = bytes.slice(i, i + len);
    i += len;
    // tags 1-5: UTF-8 text; tags 6-9: hex
    tags[tag] = tag <= 5 ? dec.decode(val) : bytesToHex(val);
  }
  return Object.keys(tags).length > 0 ? tags : null;
}

function bytesToHex(arr) {
  return Array.from(arr).map((b) => b.toString(16).padStart(2, '0')).join('');
}

// Extract ZATCA QR base64 from UBL XML.
// ZATCA standard location: cac:AdditionalDocumentReference/cbc:ID='QR' -> cac:Attachment/cbc:EmbeddedDocumentBinaryObject
// Fallback: any element with mimeCode="image/png" that looks like base64, or cbc:Note with languageID="QR".
function extractQrFromXml(xml) {
  try {
    const parser = new DOMParser();
    const doc = parser.parseFromString(xml, 'application/xml');
    const pe = doc.querySelector('parsererror');
    if (pe) return null;

    // Primary: AdditionalDocumentReference with ID=QR
    const refs = doc.querySelectorAll('AdditionalDocumentReference');
    for (const ref of refs) {
      const idEl = ref.querySelector('ID');
      if (idEl && idEl.textContent.trim() === 'QR') {
        const bin = ref.querySelector('EmbeddedDocumentBinaryObject');
        if (bin && bin.textContent.trim()) return bin.textContent.trim();
      }
    }

    // Fallback: cbc:Note with languageID="QR"
    const notes = doc.querySelectorAll('Note');
    for (const n of notes) {
      if (n.getAttribute('languageID') === 'QR' && n.textContent.trim()) return n.textContent.trim();
    }

    // Fallback: any EmbeddedDocumentBinaryObject with mimeCode image/png (QR image base64)
    const bins = doc.querySelectorAll('EmbeddedDocumentBinaryObject');
    for (const b of bins) {
      const mime = b.getAttribute('mimeCode') || '';
      if (mime.includes('png') && b.textContent.trim()) return b.textContent.trim();
    }
  } catch (_) {}
  return null;
}

function renderQrSection(xmlOrB64) {
  const wrap = $('qr-section');
  if (!wrap) return;
  const t = I18N[lang];

  // Try to extract from XML first, then treat input as raw base64
  let b64 = null;
  if (xmlOrB64 && xmlOrB64.trim().startsWith('<')) {
    b64 = extractQrFromXml(xmlOrB64);
  } else if (xmlOrB64) {
    b64 = xmlOrB64.trim();
  }

  if (!b64) {
    wrap.innerHTML = `<p class="qr-none mono">${escapeHtml(t.qrNoQr)}</p>`;
    return;
  }

  const tags = decodeTlv(b64);
  if (!tags) {
    wrap.innerHTML = `<p class="qr-none mono qr-error">${escapeHtml(t.qrInvalid)}</p>`;
    return;
  }

  const rows = Object.entries(tags).map(([tag, val]) => {
    const tagNum = parseInt(tag, 10);
    const labelEn = t.qrTagLabels[tagNum] || `Tag ${tagNum}`;
    const labelAr = t.qrTagLabelsAr[tagNum] || `وسم ${tagNum}`;
    const isBinary = tagNum >= 6;
    const displayVal = isBinary
      ? `<span class="qr-hex mono">${escapeHtml(val)}</span>`
      : `<span class="qr-text">${escapeHtml(val)}</span>`;
    return `<tr>
      <td class="qr-tag-no mono">${tagNum}</td>
      <td class="qr-label"><span class="qr-label-ar ar">${escapeHtml(labelAr)}</span><span class="qr-label-en mono">${escapeHtml(labelEn)}</span></td>
      <td class="qr-val">${displayVal}</td>
    </tr>`;
  }).join('');

  wrap.innerHTML = `<table class="qr-table" role="table" aria-label="${escapeHtml(t.qrSectionTitle)}">
    <tbody>${rows}</tbody>
  </table>`;
}

function applyLang() {
  const t = I18N[lang];
  document.documentElement.lang = lang;
  document.documentElement.dir = t.dir;
  $('lang-en').setAttribute('aria-pressed', String(lang === 'en'));
  $('lang-ar').setAttribute('aria-pressed', String(lang === 'ar'));
  document.querySelectorAll('[data-i18n]').forEach((el) => {
    const v = t[el.getAttribute('data-i18n')];
    if (typeof v === 'string') el.textContent = v;
  });
  document.querySelectorAll('[data-i18n-html]').forEach((el) => {
    const v = t[el.getAttribute('data-i18n-html')];
    if (typeof v === 'string') el.innerHTML = v;
  });
  document.querySelectorAll('[data-i18n-ph]').forEach((el) => {
    const v = t[el.getAttribute('data-i18n-ph')];
    if (typeof v === 'string') el.setAttribute('placeholder', v);
  });
  $('engine-status').textContent = engine ? t.engineReady : t.engineLoading;
  if (lastResult) renderResult(lastResult);
}

async function bootEngine() {
  try {
    const Go = globalThis.Go;
    if (typeof Go !== 'function') throw new Error('wasm_exec.js not loaded');
    const go = new Go();
    const bytes = await (await fetch('engine/engine.wasm')).arrayBuffer();
    const { instance } = await WebAssembly.instantiate(bytes, go.importObject);
    const ready = new Promise((res) => { globalThis.__zatcaReady = res; });
    go.run(instance);
    await ready;
    engine = { validateXML: (xml, p = 'peppol-bis') => JSON.parse(globalThis.zatcaValidateXML(String(xml), p)) };
    $('engine-led').classList.add('live');
    $('engine-status').textContent = I18N[lang].engineReady;
    $('run').disabled = false;
  } catch (e) {
    $('engine-status').textContent = I18N[lang].engineFail + ': ' + e.message;
  }
}

function showOnly(id) {
  ['skeleton', 'errbox', 'result'].forEach((s) => $(s).classList.toggle('hidden', s !== id));
}

async function run() {
  const xml = $('xml').value.trim();
  if (!xml) { $('xml').focus(); return; }
  if (!engine) return;
  $('run').disabled = true;
  $('run').textContent = I18N[lang].runningBtn;
  showOnly('skeleton');
  const reduce = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
  await new Promise((r) => setTimeout(r, reduce ? 0 : 450));

  let res;
  try { res = engine.validateXML(xml, 'zatca-ksa'); }
  catch (e) { res = { ok: false, error: e.message }; }

  $('run').disabled = false;
  $('run').textContent = I18N[lang].runBtn;

  if (!res || res.error || !res.report) {
    $('err-detail').textContent = (res && res.error) ? res.error : 'unknown error';
    showOnly('errbox');
    lastResult = null;
    return;
  }
  lastResult = res;
  renderResult(res);
}

function renderResult(res) {
  const t = I18N[lang];
  const report = res.report;
  const cleared = report.valid;
  const errors = report.errors || [];
  const fatal = errors.filter((e) => e.severity === 'fatal');
  const warnings = errors.filter((e) => e.severity !== 'fatal');
  const fatalCount = fatal.length;

  showOnly('result');
  const cert = $('cert');
  cert.classList.toggle('cleared', cleared);
  cert.classList.toggle('failed', !cleared);
  if (!window.matchMedia('(prefers-reduced-motion: reduce)').matches) {
    cert.classList.remove('animate'); void cert.offsetWidth; cert.classList.add('animate');
  }

  $('cert-ref').textContent = 'REF · ' + t.refFindings(errors.length);
  const today = new Date().toISOString().slice(0, 10).replace(/-/g, '');
  $('cert-serial').textContent = 'No. ZX-' + today + '-' + String((fatalCount * 7919 + errors.length * 131 + 17) % 1000000).padStart(6, '0');
  $('verdict-stamp-ar').textContent = cleared ? t.clearedStampAr : t.failedStampAr;
  $('verdict-stamp').textContent = cleared ? t.clearedStamp : t.failedStamp;
  $('verdict-title').textContent = cleared ? t.clearedTitle : t.failedTitle(fatalCount);
  $('verdict-sub').textContent = cleared ? t.clearedSub : t.failedSub;

  const tally = $('tally');
  tally.classList.toggle('cleared', cleared);
  tally.classList.toggle('failed', !cleared);
  $('tally-big').textContent = lang === 'ar' ? toAr(cleared ? 0 : fatalCount) : (cleared ? 0 : fatalCount);
  $('tally-unit').textContent = cleared ? t.tallyCleanUnit : t.tallyFatalUnit(fatalCount);

  // Grouped findings: fatal then warnings, each with a section header
  const wrap = $('findings');
  wrap.innerHTML = '';

  function buildFinding(e, globalIdx) {
    const div = document.createElement('div');
    div.className = 'finding ' + (e.severity === 'fatal' ? 'fatal' : 'warning');
    const msgEn = e.message_en || '';
    const msgAr = e.message_ar || '';
    const fixEn = e.fix_en || e.fix_ar || '';
    const fixAr = e.fix_ar || e.fix_en || '';
    const no = lang === 'ar' ? toAr(globalIdx + 1) : (globalIdx + 1);
    const tagTxt = e.severity === 'fatal'
      ? (lang === 'ar' ? 'قاتل' : 'FATAL')
      : (lang === 'ar' ? 'تحذير' : 'WARNING');
    const pathHtml = e.path ? `<span class="path mono" title="${escapeHtml(e.path)}">${escapeHtml(truncPath(e.path))}</span>` : '';
    const fixHtml = (fixEn || fixAr)
      ? `<p class="fix"><span class="fix-label mono">${escapeHtml(t.findingFix)}</span> <span${fixAr && lang === 'ar' ? ' class="ar"' : ''}>${escapeHtml(lang === 'ar' ? fixAr : fixEn)}</span></p>`
      : '';
    div.innerHTML = `
      <span class="no" aria-label="finding ${no}">${no}</span>
      <div class="finding-body">
        <div class="finding-meta">
          <span class="rid mono">${escapeHtml(e.rule_id || '')}</span>
          <span class="tag">${tagTxt}</span>
          ${pathHtml}
        </div>
        ${msgEn ? `<p class="msg msg-en">${escapeHtml(msgEn)}</p>` : ''}
        ${msgAr ? `<p class="msg msg-ar ar">${escapeHtml(msgAr)}</p>` : ''}
        ${fixHtml}
      </div>`;
    return div;
  }

  let globalIdx = 0;
  if (fatal.length > 0) {
    const hdr = document.createElement('div');
    hdr.className = 'findings-group-hdr fatal';
    hdr.textContent = t.findingsSectionFatal;
    wrap.appendChild(hdr);
    fatal.forEach((e) => { wrap.appendChild(buildFinding(e, globalIdx++)); });
  }
  if (warnings.length > 0) {
    const hdr = document.createElement('div');
    hdr.className = 'findings-group-hdr warning';
    hdr.textContent = t.findingsSectionWarning;
    wrap.appendChild(hdr);
    warnings.forEach((e) => { wrap.appendChild(buildFinding(e, globalIdx++)); });
  }

  $('findings').classList.toggle('hidden', errors.length === 0);

  // QR decoder: run against the input XML
  const inputXml = $('xml').value.trim();
  renderQrSection(inputXml);

  $('share-card').classList.add('hidden');
}

function truncPath(p) {
  if (!p) return '';
  if (p.length <= 60) return p;
  return '…' + p.slice(-57);
}

function escapeHtml(s) {
  return String(s).replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
}

// ---- shareable clearance certificate (the growth artifact) ----
const _imgCache = {};
function loadImg(src) {
  if (!_imgCache[src]) {
    _imgCache[src] = new Promise((res) => {
      const im = new Image();
      im.onload = () => res(im); im.onerror = () => res(null); im.src = src;
    });
  }
  return _imgCache[src];
}

async function buildShareImage() {
  if (!lastResult) return;
  const t = I18N[lang];
  const cleared = lastResult.report.valid;
  const canvas = $('share-canvas');
  const ctx = canvas.getContext('2d');
  const W = canvas.width, H = canvas.height;
  await (document.fonts ? document.fonts.ready : Promise.resolve());

  const PAPER = '#f2ece0', INK = '#14110c', RED = '#b00d28', GREEN = '#0a5c33';
  const accent = cleared ? GREEN : RED;

  ctx.fillStyle = PAPER; ctx.fillRect(0, 0, W, H);

  // guilloche watermark (faint, centered) + real seal (top-left)
  const guil = await loadImg('assets/guilloche.png');
  if (guil) {
    ctx.save(); ctx.globalAlpha = 0.06; const ww = 460;
    ctx.drawImage(guil, (W - ww) / 2, (H - ww) / 2, ww, ww); ctx.restore();
  }

  // heavy ink border + inner engraving frame
  ctx.strokeStyle = INK; ctx.lineWidth = 8; ctx.strokeRect(20, 20, W - 40, H - 40);
  ctx.lineWidth = 1.5; ctx.strokeRect(34, 34, W - 68, H - 68);

  // seal (girih khatim) top-left
  if (guil) { ctx.save(); ctx.globalAlpha = 0.92; ctx.drawImage(guil, 48, 56, 96, 96); ctx.restore(); }
  else drawSeal(ctx, 96, 104, 52, INK);

  // masthead
  ctx.fillStyle = INK; ctx.textAlign = 'left';
  ctx.font = '700 40px "Reem Kufi", sans-serif'; ctx.fillText('بيان حالة التخليص', 170, 96);
  ctx.fillStyle = '#4f4838'; ctx.font = '600 20px "IBM Plex Mono", monospace';
  ctx.fillText('CLEARANCE STATEMENT · ZATCA TOOLKIT', 170, 128);
  ctx.strokeStyle = INK; ctx.lineWidth = 2; ctx.beginPath(); ctx.moveTo(64, 170); ctx.lineTo(W - 64, 170); ctx.stroke();

  // the stamp
  ctx.save();
  ctx.translate(W / 2, 320); ctx.rotate(-0.10);
  ctx.strokeStyle = accent; ctx.fillStyle = accent; ctx.lineWidth = 7;
  const sw = 520, sh = 150;
  ctx.strokeRect(-sw / 2, -sh / 2, sw, sh);
  ctx.textAlign = 'center';
  ctx.font = '700 40px "Reem Kufi", sans-serif';
  ctx.fillText(cleared ? 'مُخلَّصة' : 'مرفوضة', 0, -14);
  ctx.font = '900 74px "Archivo Black", sans-serif';
  ctx.fillText(cleared ? 'CLEARED' : 'REJECTED', 0, 50);
  ctx.restore();

  // headline + tagline
  ctx.fillStyle = INK; ctx.textAlign = 'center';
  ctx.font = '900 46px "Archivo Black", sans-serif';
  ctx.fillText(t.shareHeadline(cleared), W / 2, 470);
  ctx.fillStyle = '#4f4838'; ctx.font = '600 26px "Archivo", sans-serif';
  ctx.fillText(t.shareTagline, W / 2, 516);
  ctx.fillStyle = accent; ctx.font = '600 24px "IBM Plex Mono", monospace';
  ctx.fillText('zatca-toolkit · open source · not tax advice', W / 2, 566);

  $('share-card').classList.remove('hidden');
}

function drawSeal(ctx, cx, cy, r, color) {
  ctx.save(); ctx.translate(cx, cy); ctx.strokeStyle = color; ctx.lineWidth = 1.6;
  ctx.beginPath(); ctx.arc(0, 0, r, 0, Math.PI * 2); ctx.stroke();
  ctx.beginPath(); ctx.arc(0, 0, r * 0.92, 0, Math.PI * 2); ctx.stroke();
  const R = r * 0.8;
  for (const off of [0, Math.PI / 4]) {
    ctx.beginPath();
    for (let i = 0; i < 4; i++) {
      const a = off + i * Math.PI / 2;
      const x = Math.cos(a) * R, y = Math.sin(a) * R;
      i ? ctx.lineTo(x, y) : ctx.moveTo(x, y);
    }
    ctx.closePath(); ctx.stroke();
  }
  ctx.beginPath(); ctx.arc(0, 0, r * 0.22, 0, Math.PI * 2); ctx.stroke();
  ctx.fillStyle = color; ctx.beginPath(); ctx.arc(0, 0, 3, 0, Math.PI * 2); ctx.fill();
  ctx.restore();
}

async function share() {
  await buildShareImage();
  const canvas = $('share-canvas');
  const t = I18N[lang];
  canvas.toBlob(async (blob) => {
    if (!blob) return;
    const file = new File([blob], 'clearance-statement.png', { type: 'image/png' });
    const text = t.shareHeadline(lastResult.report.valid) + ' ' + t.shareTagline;
    if (navigator.canShare && navigator.canShare({ files: [file] })) {
      try { await navigator.share({ files: [file], text }); } catch (_) {}
    }
  }, 'image/png');
}

function download() {
  const a = document.createElement('a');
  a.download = 'clearance-statement.png';
  a.href = $('share-canvas').toDataURL('image/png');
  a.click();
}

async function loadSample(which) {
  try { $('xml').value = await (await fetch(`samples/${which}.xml`)).text(); } catch (_) {}
}

function decodeManualQr() {
  const raw = $('qr-manual').value.trim();
  if (!raw) return;
  renderQrSection(raw);
}

function init() {
  $('lang-en').addEventListener('click', () => { lang = 'en'; applyLang(); });
  $('lang-ar').addEventListener('click', () => { lang = 'ar'; applyLang(); });
  $('run').addEventListener('click', run);
  $('run').disabled = true;
  $('sample-good').addEventListener('click', () => loadSample('good'));
  $('sample-bad').addEventListener('click', () => loadSample('bad'));
  $('clear-input').addEventListener('click', () => { $('xml').value = ''; showOnly(null); lastResult = null; });
  $('share').addEventListener('click', share);
  $('dl').addEventListener('click', download);
  $('cta-docs').setAttribute('href', 'https://github.com/greymoth-jp/zatca-toolkit#readme');
  $('qr-decode-btn').addEventListener('click', decodeManualQr);
  $('qr-manual').addEventListener('keydown', (e) => { if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) decodeManualQr(); });
  applyLang();
  bootEngine();
}

init();
