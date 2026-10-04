// Independent synthetic file fixtures with complete cross-reference offsets.
// The portal parses these using qpdf, rather than trusting the extension/header.
export function plainPDF(active = false): Buffer {
 const objects = ['<< /Type /Catalog /Pages 2 0 R' + (active ? ' /OpenAction 4 0 R' : '') + ' >>', '<< /Type /Pages /Kids [3 0 R] /Count 1 >>', '<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 200] /Resources << >> >>', ...(active ? ['<< /S /J#61vaScript /J#53 (app.alert\\(1\\)) >>'] : [])]
 let out = '%PDF-1.7\n'; const offsets = [0]
 for (let i = 0; i < objects.length; i++) { offsets.push(Buffer.byteLength(out)); out += `${i + 1} 0 obj\n${objects[i]}\nendobj\n` }
 const start = Buffer.byteLength(out); out += `xref\n0 ${offsets.length}\n0000000000 65535 f \n`
 for (const offset of offsets.slice(1)) out += `${String(offset).padStart(10, '0')} 00000 n \n`
 out += `trailer\n<< /Size ${offsets.length} /Root 1 0 R >>\nstartxref\n${start}\n%%EOF\n`
 return Buffer.from(out)
}
