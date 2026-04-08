<template>
  <svg viewBox="0 0 1024 1024" fill="none" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
    <defs>
      <linearGradient id="hawlerLogoBg" x1="160" y1="128" x2="864" y2="896" gradientUnits="userSpaceOnUse">
        <stop stop-color="#0F766E" />
        <stop offset="1" stop-color="#0B524E" />
      </linearGradient>
      <linearGradient id="hawlerLogoAccent" x1="300" y1="260" x2="724" y2="764" gradientUnits="userSpaceOnUse">
        <stop stop-color="#FFFFFF" />
        <stop offset="1" stop-color="#D7F4EF" />
      </linearGradient>
      <filter
        id="hawlerLogoShadow"
        x="178"
        y="178"
        width="668"
        height="668"
        filterUnits="userSpaceOnUse"
        color-interpolation-filters="sRGB"
      >
        <feFlood flood-opacity="0" result="BackgroundImageFix" />
        <feColorMatrix
          in="SourceAlpha"
          type="matrix"
          values="0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 127 0"
          result="hardAlpha"
        />
        <feOffset dy="18" />
        <feGaussianBlur stdDeviation="18" />
        <feComposite in2="hardAlpha" operator="out" />
        <feColorMatrix type="matrix" values="0 0 0 0 0.043 0 0 0 0 0.322 0 0 0 0 0.306 0 0 0 0.28 0" />
        <feBlend mode="normal" in2="BackgroundImageFix" result="effect1_dropShadow_1_1" />
        <feBlend mode="normal" in="SourceGraphic" in2="effect1_dropShadow_1_1" result="shape" />
      </filter>
    </defs>

    <circle cx="512" cy="512" r="432" fill="url(#hawlerLogoBg)" />
    <circle cx="512" cy="512" r="386" stroke="#5D8E8A" stroke-width="16" />

    <g filter="url(#hawlerLogoShadow)">
      <path
        d="M350 296C350 276.118 366.118 260 386 260H418C437.882 260 454 276.118 454 296V470H570V296C570 276.118 586.118 260 606 260H638C657.882 260 674 276.118 674 296V728C674 747.882 657.882 764 638 764H606C586.118 764 570 747.882 570 728V554H454V728C454 747.882 437.882 764 418 764H386C366.118 764 350 747.882 350 728V296Z"
        fill="url(#hawlerLogoAccent)"
      />
    </g>

    <circle cx="512" cy="512" r="432" stroke="#FDF8EF" stroke-opacity="0.2" stroke-width="8" />
  </svg>
</template>
