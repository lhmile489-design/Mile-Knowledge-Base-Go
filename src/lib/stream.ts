// Frame streams from Go Channels.
//
// A bound method that takes a *mygo.Channel streams values back to the page;
// on the page side the Channel is an async iterable. This wraps the pair into
// a small handle: `abort()` closes the channel (Go sees the closed send and
// cancels the turn) and `done` settles when both the binding call and the
// frame loop have finished.
import { Channel } from "mygo-runtime";

export interface FrameStream {
  abort(): void;
  done: Promise<void>;
}

export function openFrameStream<T>(
  start: (channel: Channel<T>) => Promise<unknown>,
  onFrame: (frame: T) => void,
): FrameStream {
  const channel = new Channel<T>();
  const reading = (async () => {
    for await (const frame of channel) onFrame(frame);
  })();
  const done = Promise.resolve()
    .then(() => start(channel))
    .then(
      () => reading,
      (error) =>
        reading.then(() => {
          throw error;
        }),
    );
  return { abort: () => channel.close(), done };
}
