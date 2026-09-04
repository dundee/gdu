import { useEffect, type ReactNode } from 'react';

interface ModalProps {
  titleId: string;
  title: string;
  onClose: () => void;
  className?: string;
  children: ReactNode;
}

// Modal is a generic dialog shell: backdrop, heading, and Escape-to-close.
// Callers only supply the content and a close callback.
export function Modal({ titleId, title, onClose, className, children }: ModalProps) {
  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        onClose();
      }
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [onClose]);

  return (
    <div className="modal-backdrop">
      <section
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
