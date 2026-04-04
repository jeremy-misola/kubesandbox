type LinkButtonProps = {
  href: string;
  variant?: 'primary' | 'secondary';
  size?: 'default' | 'large';
  children: string;
};

export function LinkButton({ href, variant = 'primary', size = 'default', children }: LinkButtonProps) {
  const className = variant === 'primary' ? 'primary-button' : 'secondary-button';
  const sizeClass = size === 'large' ? 'primary-button--large' : '';
  return (
    <a className={`${className} ${sizeClass}`} href={href}>
      {children}
    </a>
  );
}
