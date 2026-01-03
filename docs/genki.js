/**
 * Genki Cat Logic (Adapted for CloudSlash)
 * Original by savazeb/genki-cat
 */

const duration = 5; // seconds

function random(num) {
    return Math.floor(Math.random() * num);
}

function random_range(min, max) {
    return Math.random() * (max - min) + min;
}

function getRandomStyles() {
    var r = 0; // Dark/Green theme
    var g = 255;
    var b = 153;
    var mt = random(200);
    var ml = random(50);
    var dur = random(5) + 5;
    return `
  background-color: rgba(${r},${g},${b},0.3);
  color: rgba(${r},${g},${b},0.3);
  box-shadow: inset -7px -3px 10px rgba(${r},${g},${b},0.1);
  margin: ${mt}px 0 0 ${ml}px;
  animation: float ${dur}s ease-in infinite;
  border-radius: 50%;
  width: 30px; height: 40px;
  position: absolute; bottom: -50px; left: ${random(100)}vw;
  z-index: 1000;
  `;
}

// NOTE: Balloon logic simplified for pure JS/CSS float
function createBalloons(num) {
    const container = document.body;
    for (let i = 0; i < num; i++) {
        let b = document.createElement('div');
        b.className = 'balloon';
        b.style.cssText = getRandomStyles();
        container.appendChild(b);
        // Animate up
        gsap.to(b, {
            y: -window.innerHeight - 200,
            duration: random_range(5, 10),
            ease: "power1.out",
            onComplete: () => b.remove()
        });
    }
}

function createThunderstorm() {
    var thunderstormContainer = document.createElement('div');
    thunderstormContainer.id = 'thunderstorm';
    thunderstormContainer.style.position = 'fixed';
    thunderstormContainer.style.top = '0';
    thunderstormContainer.style.left = '0';
    thunderstormContainer.style.width = '100%';
    thunderstormContainer.style.height = '100%';
    thunderstormContainer.style.pointerEvents = 'none';
    thunderstormContainer.style.zIndex = '9999';
    thunderstormContainer.style.background = 'rgba(0,0,0,0.5)'; // Flash effect later?

    document.body.appendChild(thunderstormContainer);

    const rain = setInterval(function () {
        createRaindrop(thunderstormContainer);
    }, 20);

    setTimeout(function () {
        clearInterval(rain);
        thunderstormContainer.remove();
    }, duration * 1000);
}

function createRaindrop(container) {
    const raindrop = document.createElement("div");
    raindrop.className = "raindrop";
    raindrop.style.position = 'absolute';
    raindrop.style.backgroundColor = '#94A3B8';
    raindrop.style.width = '2px';
    raindrop.style.height = '15px';
    
    container.appendChild(raindrop);

    const startX = random_range(0, window.innerWidth);
    const startY = -20;
    const dropDuration = random_range(0.5, 1);

    gsap.fromTo(
        raindrop,
        { x: startX, y: startY, opacity: 1 },
        {
            x: startX - 20, // Slant
            y: window.innerHeight + 20,
            opacity: 0,
            duration: dropDuration,
            ease: "linear",
            onComplete: () => {
                raindrop.remove();
            }
        }
    );
}

function showDancingCat(img) {
    img.src = 'assets/genki/cat-state-2.gif';
    setTimeout(() => {
        img.src = 'assets/genki/cat-state-1.gif';
    }, duration * 1000);
}

function showCryingCat(img) {
    img.src = 'assets/genki/cat-state-4.gif';
    setTimeout(() => {
        img.src = 'assets/genki/cat-state-1.gif';
    }, duration * 1000);
}

// Global Text Bubble
function showMessage(msg) {
    const bubble = document.getElementById('cat-bubble');
    if(bubble) {
        bubble.innerText = msg;
        bubble.style.opacity = 1;
        setTimeout(() => bubble.style.opacity = 0.7, 3000);
    }
}

document.addEventListener("DOMContentLoaded", function () {
    // Preload
    ['assets/genki/cat-state-1.gif', 'assets/genki/cat-state-2.gif', 'assets/genki/cat-state-3.gif', 'assets/genki/cat-state-4.gif']
        .forEach(src => new Image().src = src);

    const catImg = document.getElementById('genki-cat-img');
    const btnYes = document.getElementById('btn-cat-yes');
    const btnNo = document.getElementById('btn-cat-no');

    if (!catImg || !btnYes || !btnNo) return;

    // Hover effects
    catImg.addEventListener('mouseover', () => {
        if(!catImg.src.includes('state-2') && !catImg.src.includes('state-4'))
            catImg.src = 'assets/genki/cat-state-3.gif';
    });
    catImg.addEventListener('mouseleave', () => {
         if(!catImg.src.includes('state-2') && !catImg.src.includes('state-4'))
            catImg.src = 'assets/genki/cat-state-1.gif';
    });

    // Buttons
    btnYes.addEventListener('mouseenter', () => catImg.src = 'assets/genki/cat-state-2.gif'); // Dance on hover?
    btnYes.addEventListener('mouseleave', () => catImg.src = 'assets/genki/cat-state-1.gif');

    btnYes.addEventListener('click', (e) => {
        e.preventDefault();
        showDancingCat(catImg);
        createBalloons(30);
        showMessage("Awesome! You're the best! 😸");
        
        // Scroll to pricing after a second
        setTimeout(() => {
            document.querySelector('#pricing-cards').scrollIntoView({behavior: 'smooth'});
        }, 1500);
    });

    btnNo.addEventListener('click', (e) => {
        e.preventDefault();
        showCryingCat(catImg);
        createThunderstorm();
        showMessage("Aww... maybe next time? 😿");
    });
});
