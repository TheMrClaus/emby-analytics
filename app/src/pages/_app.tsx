import type { AppProps } from "next/app";
import "../styles/globals.css";
import { NowPlayingProvider } from "../contexts/NowPlayingContext";
import { MultiServerProvider } from "../contexts/MultiServerContext";

export default function App({ Component, pageProps }: AppProps) {
  return (
    <MultiServerProvider>
      <NowPlayingProvider>
        <Component {...pageProps} />
      </NowPlayingProvider>
    </MultiServerProvider>
  );
}
