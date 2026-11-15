import { useEffect, useRef, type RefObject } from 'react';

// useLatest keeps a ref pointing at the most recent value it was called with.
//
// It exists for the same reason in both places it is used: an async load
// started for one directory can resolve after the user has navigated to
// another, and the callback needs to compare against where the user is *now*
// rather than the path captured when it started. Reading that through a ref
// keeps the callback out of the effect's dependency list, which would
// otherwise restart the very load it is guarding.
export function useLatest<T>(value: T): RefObject<T> {
  const ref = useRef(value);
  useEffect(() => {
    ref.current = value;
  }, [value]);
  return ref;
}
