const commands = [
    { text: "./cloudslash scan --profile production", type: "cmd", delay: 800 },
    { text: "Initializing Zero-Trust Graph...", type: "info", delay: 400 },
    { text: "Scanning Region: us-east-1", type: "info", delay: 200 },
    { text: "Analyzing 452 Resources...", type: "info", delay: 600 },
    { text: "⚠ DETECTED: 12 Zombie EBS Volumes (stopped > 30d)", type: "warn", delay: 300 },
    { text: "⚠ DETECTED: 5 NAT Gateways (Traffic < 1GB)", type: "warn", delay: 300 },
    { text: "⚠ DETECTED: 2 Abandoned ELBs", type: "warn", delay: 300 },
    { text: "Calculating Waste...", type: "info", delay: 500 },
    { text: "POTENTIAL SAVINGS: $842/month", type: "success", delay: 1000 },
    { text: "", type: "cmd", delay: 2000 } // Pause before restart
];

const terminalBody = document.getElementById('term-body');

async function typeWriter(text, element) {
    for (let char of text) {
        element.innerHTML += char;
        await new Promise(r => setTimeout(r, Math.random() * 30 + 20));
    }
}

async function runTerminal() {
    terminalBody.innerHTML = '';
    
    for (let cmd of commands) {
        const line = document.createElement('div');
        line.className = 'line ' + cmd.type;
        terminalBody.appendChild(line);

        if (cmd.type === 'cmd') {
            line.innerHTML = '<span class="prompt">user@cloudslash:~$</span> ';
            const span = document.createElement('span');
            line.appendChild(span);
            await typeWriter(cmd.text, span);
        } else {
            line.innerText = cmd.text;
        }

        // Auto scroll
        terminalBody.scrollTop = terminalBody.scrollHeight;
        
        await new Promise(r => setTimeout(r, cmd.delay));
    }

    // Loop
    setTimeout(runTerminal, 1000);
}

document.addEventListener('DOMContentLoaded', () => {
    runTerminal();
});

// Glitch Effect for Logo
const logoText = document.querySelector('.logo-text');
// Add hover listener if needed
// logoText.addEventListener('mouseover', ...);
