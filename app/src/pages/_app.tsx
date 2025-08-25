import type { AppProps } from "next/app";
import "../styles/globals.css";
import { NowPlayingProvider } from "../contexts/NowPlayingContext";

export default function App({ Component, pageProps }: AppProps) {
  return (
    <NowPlayingProvider>
      <Component {...pageProps} />
    </NowPlayingProvider>
  );
}
