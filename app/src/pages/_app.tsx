import type { AppProps } from "next/app";
import "../styles/globals.css";
import { NowPlayingProvider } from "../contexts/NowPlayingContext";
import { MultiServerProvider } from "../contexts/MultiServerContext";
import { LibraryServerProvider } from "../contexts/LibraryServerContext";

export default function App({ Component, pageProps }: AppProps) {
  return (
    <LibraryServerProvider>
      <MultiServerProvider>
        <NowPlayingProvider>
          <Component {...pageProps} />
        </NowPlayingProvider>
      </MultiServerProvider>
    </LibraryServerProvider>
  );
}
