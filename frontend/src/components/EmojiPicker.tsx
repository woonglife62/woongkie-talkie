import { useRef, useEffect } from 'react';

const EMOJIS = [
  '😀','😂','😍','🥰','😎','🤔','😅','😭',
  '🎉','👍','👎','❤️','🔥','✨','💯','🙏',
  '😊','😋','🤣','😆','😉','🥳','🤩','😇',
  '👋','🤝','💪','🖐️','✌️','🤞','🤙','👏',
  '🐱','🐶','🦊','🐼','🐨','🦁','🐯','🐸',
  '🍕','🍔','🍣','🍜','🍰','☕','🍺','🎂',
  '⚽','🏀','🎮','🎵','🎸','📚','💻','🚀',
];

interface EmojiPickerProps {
  onSelect: (emoji: string) => void;
  onClose: () => void;
}

export function EmojiPicker({ onSelect, onClose }: EmojiPickerProps) {
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        onClose();
      }
    };
    document.addEventListener('mousedown', handler);
    return () => document.removeEventListener('mousedown', handler);
  }, [onClose]);

  return (
    <div className="emoji-picker" ref={ref}>
      <div className="emoji-grid">
        {EMOJIS.map((emoji) => (
          <button
            key={emoji}
            className="emoji-btn"
            onClick={() => onSelect(emoji)}
            type="button"
            title={emoji}
          >
            {emoji}
          </button>
        ))}
      </div>
    </div>
  );
}
