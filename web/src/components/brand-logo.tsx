import type { FC } from "react";

interface BrandLogoProps {
  className?: string;
  size?: number;
}

export const BrandDelta: FC<BrandLogoProps> = ({ className = "w-8 h-8", size = 32 }) => {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 64 64"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      className={className}
    >
      <defs>
        <linearGradient id="brandPrismA" x1="32" y1="8" x2="12" y2="52" gradientUnits="userSpaceOnUse">
          <stop offset="0%" stopColor="#00F0FF" />
          <stop offset="100%" stopColor="#006680" />
        </linearGradient>
        <linearGradient id="brandPrismB" x1="32" y1="8" x2="52" y2="52" gradientUnits="userSpaceOnUse">
          <stop offset="0%" stopColor="#00F0FF" />
          <stop offset="100%" stopColor="#003B4A" />
        </linearGradient>
      </defs>
      <rect width="64" height="64" rx="14" fill="#09090B" stroke="#27272A" strokeWidth="2" />
      <path d="M32 12 L14 52 L32 44 Z" fill="url(#brandPrismA)" />
      <path d="M32 12 L50 52 L32 44 Z" fill="url(#brandPrismB)" />
      <path d="M32 12 L32 44" stroke="#FFFFFF" strokeWidth="1.5" strokeLinecap="round" opacity="0.9" />
      <path d="M24 47.5 L32 44 L40 47.5" stroke="#00F0FF" strokeWidth="1.5" strokeLinecap="round" />
    </svg>
  );
};
