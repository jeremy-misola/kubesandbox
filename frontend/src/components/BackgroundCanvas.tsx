import React, { useEffect, useRef } from 'react';

/**
 * Full-screen animated canvas background.
 * - Draws a subtle interactive grid that warps toward the mouse
 * - Three floating radial orbs in the design palette (lime, cyan, violet)
 * - Faint horizontal scanlines for a CRT depth effect
 */
export const BackgroundCanvas: React.FC = () => {
  const canvasRef = useRef<HTMLCanvasElement>(null);

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;

    const ctx = canvas.getContext('2d');
    if (!ctx) return;

    let animationFrameId: number;
    let width  = (canvas.width  = window.innerWidth);
    let height = (canvas.height = window.innerHeight);

    const mouse = { x: width / 2, y: height / 2, tx: width / 2, ty: height / 2 };

    const handleResize = () => {
      width  = canvas.width  = window.innerWidth;
      height = canvas.height = window.innerHeight;
    };
    const handleMouseMove = (e: MouseEvent) => { mouse.tx = e.clientX; mouse.ty = e.clientY; };

    window.addEventListener('resize', handleResize);
    window.addEventListener('mousemove', handleMouseMove);

    // Three orbs — lime / cyan / violet  (matching --primary / --accent / --secondary)
    const orbs = [
      { x: width * 0.18, y: height * 0.22, r: 280, color: 'rgba(164, 255, 79,  0.045)', vx: 0.4, vy: 0.28 },
      { x: width * 0.78, y: height * 0.30, r: 360, color: 'rgba(34,  211, 238, 0.045)', vx: -0.3, vy: 0.35 },
      { x: width * 0.48, y: height * 0.72, r: 310, color: 'rgba(129, 140, 248, 0.035)', vx: 0.25, vy: -0.4 },
    ];

    let time = 0;

    const render = () => {
      time += 0.0018;

      // Background fill
      ctx.fillStyle = '#080b14';
      ctx.fillRect(0, 0, width, height);

      // Smooth mouse lerp
      mouse.x += (mouse.tx - mouse.x) * 0.048;
      mouse.y += (mouse.ty - mouse.y) * 0.048;

      // ── Warping grid ──
      ctx.strokeStyle = 'rgba(255, 255, 255, 0.014)';
      ctx.lineWidth   = 1;
      const gs = 50; // grid spacing

      for (let gx = 0; gx < width; gx += gs) {
        ctx.beginPath();
        for (let gy = 0; gy <= height; gy += 18) {
          const dx   = gx - mouse.x;
          const dy   = gy - mouse.y;
          const dist = Math.sqrt(dx * dx + dy * dy) || 1;
          const pull = Math.max(0, 1 - dist / 280) * 14;
          const ox   = -(dx / dist) * pull;
          gy === 0 ? ctx.moveTo(gx + ox, gy) : ctx.lineTo(gx + ox, gy);
        }
        ctx.stroke();
      }

      for (let gy = 0; gy < height; gy += gs) {
        ctx.beginPath();
        for (let gx = 0; gx <= width; gx += 18) {
          const dx   = gx - mouse.x;
          const dy   = gy - mouse.y;
          const dist = Math.sqrt(dx * dx + dy * dy) || 1;
          const pull = Math.max(0, 1 - dist / 280) * 14;
          const oy   = -(dy / dist) * pull;
          gx === 0 ? ctx.moveTo(gx, gy + oy) : ctx.lineTo(gx, gy + oy);
        }
        ctx.stroke();
      }

      // ── Floating orbs ──
      orbs.forEach(orb => {
        orb.x += orb.vx + Math.sin(time + orb.r * 0.01) * 0.18;
        orb.y += orb.vy + Math.cos(time + orb.r * 0.01) * 0.18;

        // Boundary bounce with slight damping
        if (orb.x < 0 || orb.x > width)  { orb.vx *= -1; orb.x = Math.max(0, Math.min(width,  orb.x)); }
        if (orb.y < 0 || orb.y > height) { orb.vy *= -1; orb.y = Math.max(0, Math.min(height, orb.y)); }

        // Mouse repulsion
        const dx   = mouse.x - orb.x;
        const dy   = mouse.y - orb.y;
        const dist = Math.sqrt(dx * dx + dy * dy) || 1;
        if (dist < 380) {
          orb.x -= (dx / dist) * 0.55;
          orb.y -= (dy / dist) * 0.55;
        }

        const grad = ctx.createRadialGradient(orb.x, orb.y, 0, orb.x, orb.y, orb.r);
        grad.addColorStop(0, orb.color);
        grad.addColorStop(1, 'rgba(8, 11, 20, 0)');
        ctx.fillStyle = grad;
        ctx.beginPath();
        ctx.arc(orb.x, orb.y, orb.r, 0, Math.PI * 2);
        ctx.fill();
      });

      // ── Faint scanlines ──
      ctx.fillStyle = 'rgba(255, 255, 255, 0.0025)';
      for (let sy = 0; sy < height; sy += 4) {
        ctx.fillRect(0, sy, width, 1);
      }

      animationFrameId = requestAnimationFrame(render);
    };

    render();

    return () => {
      window.removeEventListener('resize', handleResize);
      window.removeEventListener('mousemove', handleMouseMove);
      cancelAnimationFrame(animationFrameId);
    };
  }, []);

  return (
    <canvas
      ref={canvasRef}
      className="fixed inset-0 w-full h-full pointer-events-none"
      style={{ zIndex: -10 }}
    />
  );
};
