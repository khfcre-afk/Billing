import type { Config } from "tailwindcss";
import animate from "tailwindcss-animate";

export default {
  darkMode: ["class"],
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: {
    container: {
      center: true,
      padding: "1rem",
      screens: { "2xl": "1280px" },
    },
    extend: {
      fontFamily: {
        display: ["'Space Grotesk'", "system-ui", "sans-serif"],
        body: ["Inter", "system-ui", "sans-serif"],
      },
      colors: {
        bg: "#0A0A0F",
        surface: "#12121A",
        "surface-2": "#1A1A24",
        border: "#2A2A38",
        text: "#EDEDF2",
        muted: "#8B8B9A",
        accent: {
          DEFAULT: "#7C5CFF",
          foreground: "#ffffff",
        },
        "accent-2": "#22D3EE",
        success: "#22C55E",
        danger: "#EF4444",
      },
      borderRadius: {
        xl: "14px",
        "2xl": "20px",
      },
      boxShadow: {
        glow: "0 0 60px -12px rgba(124, 92, 255, 0.45)",
      },
      backgroundImage: {
        "hero-gradient":
          "radial-gradient(1200px 600px at 20% -10%, rgba(124,92,255,0.35), transparent 60%), radial-gradient(800px 400px at 90% 10%, rgba(34,211,238,0.25), transparent 60%), linear-gradient(180deg, #0A0A0F 0%, #0A0A0F 100%)",
      },
      keyframes: {
        "fade-in-up": {
          "0%": { opacity: "0", transform: "translateY(8px)" },
          "100%": { opacity: "1", transform: "translateY(0)" },
        },
      },
      animation: {
        "fade-in-up": "fade-in-up 300ms ease-out both",
      },
    },
  },
  plugins: [animate],
} satisfies Config;
