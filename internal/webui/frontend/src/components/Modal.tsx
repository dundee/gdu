import { useEffect, useRef, type ReactNode } from 'react';

interface ModalProps {
  titleId: string;
  title: string;
  onClose: () => void;
  className?: string;
  children: ReactNode;
}

// Modal is a generic dialog shell: backdrop, heading, Escape-to-close, and
// Left/Right arrow roving focus between its buttons (Tab already does this,
// but arrow keys are the more familiar way to move between a dialog's
// actions without leaving the keyboard's home row).
export function Modal({ titleId, title, onClose, className, children }: ModalProps) {
  const rootRef = useRef<HTMLElement>(null);

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        onClose();
        return;
      }
      if (event.key !== 'ArrowLeft' && event.key !== 'ArrowRight') {
        return;
      }
      const buttons = Array.from(
        rootRef.current?.querySelectorAll<HTMLButtonElement>('button:not(:disabled)') ?? [],
      );
      const activeIndex = buttons.indexOf(document.activeElement as HTMLButtonElement);
      if (activeIndex === -1) {
        return;
      }
      event.preventDefault();
      const delta = event.key === 'ArrowRight' ? 1 : -1;
      buttons[(activeIndex + delta + buttons.length) % buttons.length]?.focus();
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [onClose]);

  return (
    <div className="modal-backdrop">
      <section
        ref={rootRef}
        className={`modal ${className ?? ''}`}
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
      >
        <h2 id={titleId}>{title}</h2>
        {children}
      </section>
    </div>
  );
}
