import React, { useState, useEffect, useRef } from 'react';

interface AnimeTextProps {
  text: string;
  className?: string;
  triggerOnHover?: boolean;
  triggerOnMount?: boolean;
  speed?: number;
}

export const AnimeText: React.FC<AnimeTextProps> = ({
  text,
  className = '',
  triggerOnHover = true,
  triggerOnMount = true,
  speed = 30
}) => {
  const [displayText, setDisplayText] = useState<string>(text);
  const isRunning = useRef(false);
  const chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*()_+-=[]{}|;:,.<>?';

  const scramble = () => {
    if (isRunning.current) return;
    isRunning.current = true;

    let iterations = 0;
    const interval = setInterval(() => {
      setDisplayText(
        text
          .split('')
          .map((char, index) => {
            if (char === ' ') return ' ';
            if (index < iterations) return text[index];
            return chars[Math.floor(Math.random() * chars.length)];
          })
          .join('')
      );

      if (iterations >= text.length) {
        clearInterval(interval);
        isRunning.current = false;
        setDisplayText(text);
      }
      
      // Reveal 1 character every few ticks
      iterations += 1 / 3;
    }, speed);
  };

  useEffect(() => {
    if (triggerOnMount) {
      scramble();
    } else {
      setDisplayText(text);
    }
  }, [text, triggerOnMount]);

  return (
    <span
      className={`inline-block ${className}`}
      onMouseEnter={() => {
        if (triggerOnHover) scramble();
      }}
    >
      {displayText}
    </span>
  );
};
