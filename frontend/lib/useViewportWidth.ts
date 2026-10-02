import { useSyncExternalStore } from 'react';

const SERVER_WIDTH = 1280;

function subscribe(onChange: () => void) {
  window.addEventListener('resize', onChange);
  return () => window.removeEventListener('resize', onChange);
}

export function useViewportWidth() {
  return useSyncExternalStore(
    subscribe,
    () => window.innerWidth,
    () => SERVER_WIDTH,
  );
}
