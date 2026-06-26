/** @type {import('tailwindcss').Config} */
export default {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        ink: "#10212b",
        mist: "#eef5f2",
        accent: "#0d7a5f",
        warn: "#d94841",
        gold: "#d8a319"
      }
    },
  },
  plugins: [],
};
