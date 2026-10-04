export function Neighbourhood({ className = '' }: { className?: string }) {
  return <svg className={className} aria-hidden="true" viewBox="0 0 620 430" fill="none">
    <defs>
      <linearGradient id="ground" x1="310" y1="230" x2="310" y2="430" gradientUnits="userSpaceOnUse"><stop stopColor="#375347" /><stop offset="1" stopColor="#203c31" /></linearGradient>
      <pattern id="windows" width="33" height="38" patternUnits="userSpaceOnUse"><rect x="12" y="10" width="11" height="19" rx="5.5" fill="#4b6151" /></pattern>
      <pattern id="light-windows" width="32" height="36" patternUnits="userSpaceOnUse"><rect x="9" y="8" width="13" height="18" rx="6.5" fill="#faf3d9" /><path d="M15.5 8v18" stroke="#748566" /></pattern>
      <linearGradient id="cream" x1="0" y1="0" x2="180" y2="250"><stop stopColor="#f6ecd5" /><stop offset="1" stopColor="#c7c6a4" /></linearGradient>
    </defs>
    <circle cx="425" cy="125" r="105" fill="#dfe7ad" opacity=".13" />
    <circle cx="425" cy="125" r="77" stroke="#e5ecb9" opacity=".15" />
    <path d="M64 337 303 211l264 126-249 101Z" fill="url(#ground)" />
    <path d="m83 355 234 62 222-80" stroke="#9fac8c" strokeWidth="1.5" opacity=".5" />
    <path d="m153 330 157 75 166-90" stroke="#d8d9b5" strokeWidth="20" />
    <path d="m153 330 157 75 166-90" stroke="#ede6cc" strokeWidth="12" />
    <path d="m270 251 71-37 100 48v104l-100 51-71-37Z" fill="#53674f" />
    <path d="m341 214 100 48v104l-100 51Z" fill="#9da783" />
    <path d="m270 251 71-37 100 48-71 37Z" fill="#c6cbaa" />
    <path d="m349 263 84 39v55l-84 43Z" fill="url(#light-windows)" />
    <path d="m324 303 17 8v87l-17-8Z" fill="#cbd1ad" />
    <path d="m183 174 78-39 90 44v162l-90 43-78-38Z" fill="#adac91" />
    <path d="m261 135 90 44v162l-90 43Z" fill="url(#cream)" />
    <path d="m183 174 78-39 90 44-78 39Z" fill="#f2edd7" />
    <path d="m274 210 65 31v92l-65 32Z" fill="url(#windows)" />
    <path d="m212 197 22 11v129l-22-11Z" fill="#667761" />
    <path d="m274 300 18 9v49l-18 9Z" fill="#243d31" />
    <path d="m371 162 53-27 81 39v132l-81 40-53-26Z" fill="#aaaea2" />
    <path d="m424 135 81 39v132l-81 40Z" fill="#cad1bd" />
    <path d="m371 162 53-27 81 39-53 27Z" fill="#eff0dc" />
    <path d="m436 204 58 28v65l-58 29Z" fill="url(#windows)" />
    <path d="m387 182 13 7v110l-13-6Z" fill="#768a70" />
    <g><path d="M151 344v-51" stroke="#c0c9a3" strokeWidth="6" /><ellipse cx="151" cy="281" rx="34" ry="47" fill="#899b64" /><path d="M151 324v-60m0 28-17-14m17 27 16-16" stroke="#556847" strokeWidth="2" /></g>
    <g><path d="M498 347v-40" stroke="#bcc59a" strokeWidth="5" /><ellipse cx="498" cy="294" rx="27" ry="39" fill="#bec983" /><path d="M498 325v-52m0 20-11-10m11 19 13-12" stroke="#829258" strokeWidth="2" /></g>
    <g><path d="M387 404v-29" stroke="#b4be93" strokeWidth="5" /><ellipse cx="387" cy="361" rx="23" ry="32" fill="#738a57" /></g>
    <ellipse cx="205" cy="381" rx="18" ry="8" fill="#71824f" /><ellipse cx="461" cy="362" rx="16" ry="7" fill="#a8b47a" />
    <path d="m112 200 3-7 3 7 7 3-7 3-3 7-3-7-7-3Z" fill="#dbe7a9" />
    <path d="m534 111 2-5 2 5 5 2-5 2-2 5-2-5-5-2Z" fill="#dbe7a9" />
  </svg>
}

