/**
 * CloudSlash Smart Cats
 * Logic:
 * - Spawns cats that walk in/out of screen.
 * - Randomly decides to be "confused" (scribble).
 * - Detects button hovers to become "happy" or "peek" near the button.
 */

class CatSystem {
    constructor() {
        this.cats = [];
        this.container = document.body;
        this.buttons = document.querySelectorAll('.btn-secondary-full, .btn-primary-full');
        this.init();
    }

    init() {
        // Spawn standard walking cat
        this.spawnTimer = setInterval(() => this.trySpawnCat(), 8000); // Try every 8s
        
        // Listen for button interaction
        this.buttons.forEach(btn => {
            btn.addEventListener('mouseenter', (e) => this.handleButtonHover(e.target));
            btn.addEventListener('mouseleave', () => this.handleButtonLeave());
        });

        // Initial spawn
        setTimeout(() => this.trySpawnCat(), 1000);
    }

    createSVG(state = 'walk') {
        const colors = ['#1E293B', '#334155', '#475569'];
        const color = colors[Math.floor(Math.random() * colors.length)];
        
        // Basic Cat SVG parts
        const body = `<path d="M10 50 L30 10 L50 50" fill="${color}" stroke="#000" stroke-width="2"/>
                      <path d="M70 50 L90 10 L110 50" fill="${color}" stroke="#000" stroke-width="2"/>
                      <circle cx="60" cy="80" r="40" fill="${color}" stroke="#000" stroke-width="2"/>`; // Head

        const eyesNormal = `<ellipse cx="45" cy="70" rx="6" ry="10" fill="#00FF99"/><ellipse cx="75" cy="70" rx="6" ry="10" fill="#00FF99"/>
                            <circle cx="45" cy="70" r="2" fill="#000"/><circle cx="75" cy="70" r="2" fill="#000"/>`;
        
        const eyesConfused = `<path d="M40 70 L50 70" stroke="#00FF99" stroke-width="3"/><path d="M70 70 L80 70" stroke="#00FF99" stroke-width="3"/>
                              <path d="M45 65 L45 75" stroke="#00FF99" stroke-width="3"/><path d="M75 65 L75 75" stroke="#00FF99" stroke-width="3"/>`; // Cross eyes
        
        const eyesHappy = `<path d="M40 70 Q45 65 50 70" fill="none" stroke="#00FF99" stroke-width="3"/>
                           <path d="M70 70 Q75 65 80 70" fill="none" stroke="#00FF99" stroke-width="3"/>`; // ^ ^ eyes

        const scribble = `<path class="scribble" d="M30 20 Q40 0 50 20 Q60 40 70 20 Q80 0 90 20" fill="none" stroke="#F472B6" stroke-width="2" style="opacity:0;"/>`;

        return `<svg viewBox="0 0 120 120" style="width:100px; height:auto; overflow:visible;">
            ${scribble}
            <g class="cat-body">
                ${body}
                <g class="cat-face-normal" style="opacity: 1">${eyesNormal}</g>
                <g class="cat-face-confused" style="opacity: 0">${eyesConfused}</g>
                <g class="cat-face-happy" style="opacity: 0">${eyesHappy}</g>
                <path d="M55 85 L65 85 L60 92 Z" fill="#F472B6"/>
                <path d="M20 80 L5 75" stroke="#94A3B8" stroke-width="2"/><path d="M20 85 L5 85" stroke="#94A3B8" stroke-width="2"/>
                <path d="M100 80 L115 75" stroke="#94A3B8" stroke-width="2"/><path d="M100 85 L115 85" stroke="#94A3B8" stroke-width="2"/>
            </g>
        </svg>`;
    }

    trySpawnCat() {
        if (this.cats.length > 2) return; // Max 2 cats at once

        const catEl = document.createElement('div');
        catEl.innerHTML = this.createSVG();
        catEl.style.position = 'fixed';
        catEl.style.bottom = '-20px';
        catEl.style.zIndex = '90';
        catEl.style.transition = 'transform 0.5s ease';
        
        // Random side start
        const startLeft = Math.random() > 0.5;
        catEl.style.left = startLeft ? '-100px' : '100vw';
        catEl.style.transform = startLeft ? 'scaleX(1)' : 'scaleX(-1)'; // Face direction
        
        this.container.appendChild(catEl);

        const catVals = { el: catEl, state: 'walking', x: startLeft ? -100 : window.innerWidth, facingRight: startLeft };
        this.cats.push(catVals);

        this.animateCat(catVals);
    }

    animateCat(cat) {
        if (!cat.el.parentNode) return;

        // walk
        const speed = 2 + Math.random() * 2;
        cat.x += cat.facingRight ? speed : -speed;
        cat.el.style.left = `${cat.x}px`;

        // Bounce effect
        cat.el.style.bottom = `${Math.sin(Date.now() / 100) * 5 - 10}px`;

        // Random event: Confusion
        if (Math.random() < 0.005 && cat.state !== 'confused') {
            this.setConfused(cat);
        }

        // Out of bounds check
        if ((cat.facingRight && cat.x > window.innerWidth) || (!cat.facingRight && cat.x < -150)) {
            cat.el.remove();
            this.cats = this.cats.filter(c => c !== cat);
            return;
        }

        if (cat.state !== 'stopped') {
            requestAnimationFrame(() => this.animateCat(cat));
        }
    }

    setConfused(cat) {
        cat.state = 'confused';
        // Show scribble
        const scribble = cat.el.querySelector('.scribble');
        const normal = cat.el.querySelector('.cat-face-normal');
        const confused = cat.el.querySelector('.cat-face-confused');
        
        scribble.style.opacity = '1';
        scribble.style.animation = 'scribble-anim 0.5s infinite';
        normal.style.opacity = '0';
        confused.style.opacity = '1';

        // Pause walking
        const oldState = cat.state;
        cat.state = 'stopped';

        setTimeout(() => {
            scribble.style.opacity = '0';
            scribble.style.animation = 'none';
            normal.style.opacity = '1';
            confused.style.opacity = '0';
            cat.state = 'walking';
            this.animateCat(cat);
        }, 3000); // 3 seconds confusion
    }

    handleButtonHover(btn) {
        // Spawn a peeking cat near the button
        // Logic: Find closest edge of screen and peek out
        const rect = btn.getBoundingClientRect();
        
        // Remove old peeker if exists
        if (this.peeker) this.peeker.remove();

        this.peeker = document.createElement('div');
        this.peeker.innerHTML = this.createSVG();
        this.peeker.style.position = 'fixed';
        this.peeker.style.zIndex = '100';
        this.peeker.style.transition = 'all 0.5s cubic-bezier(0.175, 0.885, 0.32, 1.275)';
        
        // Face happy
        this.peeker.querySelector('.cat-face-normal').style.opacity = '0';
        this.peeker.querySelector('.cat-face-happy').style.opacity = '1';

        // Position: Peek from bottom aligned with button x
        this.peeker.style.left = `${rect.left + rect.width / 2 - 50}px`;
        this.peeker.style.bottom = '-100px'; 
        
        this.container.appendChild(this.peeker);
        
        // Animate up
        setTimeout(() => {
            this.peeker.style.bottom = '0px';
        }, 50);
    }

    handleButtonLeave() {
        if (this.peeker) {
            this.peeker.style.bottom = '-150px';
            setTimeout(() => this.peeker.remove(), 500);
        }
    }
}

// Styles for scribble
const style = document.createElement('style');
style.innerHTML = `
@keyframes scribble-anim {
    0% { transform: translateY(0) rotate(0deg); }
    25% { transform: translateY(-5px) rotate(10deg); }
    75% { transform: translateY(-5px) rotate(-10deg); }
    100% { transform: translateY(0) rotate(0deg); }
}`;
document.head.appendChild(style);

// Start
document.addEventListener('DOMContentLoaded', () => new CatSystem());
