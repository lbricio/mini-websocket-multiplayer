const ws = new WebSocket('ws://192.168.0.107:8080/ws'); // UPDATE!!!

const tileset = new Image();
let tileSize = 32;

fetch("maps/test.json")
    .then(res => res.json())
    .then(json => {
        tileMapData = json;
        tileset.src = tileMapData.tileset;
    });

function drawMap(cameraX, cameraY) {
    if (!tileMapData) return;
    if (!tileset.complete) return;

    const layer = tileMapData.layers.find(l => l.name === "ground");

    const cols = tileMapData.width;
    const rows = tileMapData.height;

    const tilesPerRow = tileset.width / tileSize;

    const scaledTileW = canvas.width / tilesOnScreenX;
    const scaledTileH = canvas.height / tilesOnScreenY;

    for (let y = 0; y < rows; y++) {
        for (let x = 0; x < cols; x++) {
            const i = y * cols + x;
            const tileIndex = layer.data[i];

            const sx = (tileIndex % tilesPerRow) * tileSize;
            const sy = Math.floor(tileIndex / tilesPerRow) * tileSize;

            const dx = x * scaledTileW - cameraX;
            const dy = y * scaledTileH - cameraY;

            ctx.drawImage(
                tileset,
                sx, sy, tileSize, tileSize,         // fonte (tileset)
                dx, dy, scaledTileW, scaledTileH    // destino (canvas)
            );
        }
    }
}

const spriteSheet = new Image();
spriteSheet.src = 'charset/sprite.png';

const frameWidth = 48;
const frameHeight = 72;
const framesPerAnim = 3;
const directions = ["down", "left", "right", "up"];
const charactersPerRow = 4;

const tilesOnScreenX = 24;
const tilesOnScreenY = 15;

const movementSpeed = 300;
const animationStyle = [0,1,2,1];

const canvas = document.getElementById('game');
const ctx = canvas.getContext('2d');

let tileMapData = null;

function resizeCanvas() {
    const canvas = document.getElementById('game');

    const scaleX = window.innerWidth / canvas.width;
    const scaleY = window.innerHeight / canvas.height;
    const scale = Math.min(scaleX, scaleY, 1);

    canvas.style.transform = `scale(${scale})`;
    canvas.style.transformOrigin = 'top center';

    // Remove o ajuste da largura do container para deixar o flex funcionar corretamente
    // container.style.width = `${canvas.width * scale}px`;
    // container.style.height = `${canvas.height * scale}px`;
}

window.addEventListener('resize', resizeCanvas);
window.addEventListener('load', resizeCanvas);

let players = [];
let myPlayerID = null;

ws.onmessage = function(event) {
    const msg = JSON.parse(event.data);

    if (msg.type === 'init') {
        myPlayerID = msg.id;
    }

    if (msg.type === 'state') {
        players = msg.players.map(p => {
            let existing = players.find(oldP => oldP.id === p.id);
            return {
                ...p,
                x_img: existing ? existing.x_img : p.x * tileSize,
                y_img: existing ? existing.y_img : p.y * tileSize,
                animFrame: existing ? existing.animFrame : 0,
                animTime: existing ? existing.animTime : 0,
                chatTimestamp: existing?.chatText !== p.chatText ? performance.now() : existing?.chatTimestamp || 0
            };
        });
    }

    if (msg.type === 'chat') {
        const sender = players.find(p => p.id === msg.id);
        if (sender) {
            sender.chatText = msg.text;

            // Remover texto após 10 segundos
            setTimeout(() => {
                if (sender.chatText === msg.text) {
                    sender.chatText = '';
                }
            }, 15000);
        }
    }
};

document.getElementById('chatInput').addEventListener('keydown', function(event) {
    if (event.key === 'Enter' && this.value.trim() !== '') {
        const msg = this.value.trim().slice(0, 60);
        ws.send(JSON.stringify({ type: 'chat', text: msg }));
        this.value = '';
    }
});

function sendMove(dir) {
    ws.send(JSON.stringify({ type: 'move', dir: dir }));
}

const directionsInput = ["up", "down", "left", "right"];
directionsInput.forEach(dir => {
    const btn = document.getElementById(dir);
    btn.addEventListener('mousedown', () => sendMove(dir));
    btn.addEventListener('touchstart', e => { e.preventDefault(); sendMove(dir); }, { passive: false });
});


const keysPressed = {};

const chatInput = document.getElementById('chatInput');

document.addEventListener('keydown', function(event) {
    // Se foco estiver no chatInput, ignorar controle de movimento
    if (document.activeElement.id === 'chatInput') {
        // Tratamento para desfocar com Tab enquanto está no input
        if (event.key === 'Tab') {
            event.preventDefault();
            chatInput.blur();
        }
        return;
    }

    // Se não estiver no chatInput
    if (event.key === 'Tab') {
        event.preventDefault();
        chatInput.focus();
        return;
    }

    // Controla teclas de movimento
    keysPressed[event.key.toLowerCase()] = true;
});

document.addEventListener('keyup', function(event) {
    keysPressed[event.key.toLowerCase()] = false;
});

setInterval(function() {
    if (keysPressed['w']) sendMove('up');
    if (keysPressed['s']) sendMove('down');
    if (keysPressed['a']) sendMove('left');
    if (keysPressed['d']) sendMove('right');
}, 50);

chatInput.addEventListener('keydown', function(event) {
    if (event.key === 'Enter' && this.value.trim() !== '') {
        const msg = this.value.trim().slice(0, 60);
        ws.send(JSON.stringify({ type: 'chat', text: msg }));
        this.value = '';
        this.blur(); // desfocar após enviar
    }
});


function lerp(a, b, t) {
    return a + (b - a) * t;
}

function updatePlayerPositions(deltaTime) {
    for (let p of players) {
        const targetX = p.x * tileSize;
        const targetY = p.y * tileSize;

        const dx = targetX - p.x_img;
        const dy = targetY - p.y_img;

        const distance = Math.sqrt(dx * dx + dy * dy);

        if (distance > 0.5) {
            const maxStep = (movementSpeed * deltaTime) / 1000;  // deltaTime está em ms
            const step = Math.min(distance, maxStep);
            const ratio = step / distance;
            p.x_img += dx * ratio;
            p.y_img += dy * ratio;
        } else {
            p.x_img = targetX;
            p.y_img = targetY;
        }
    }
}

function isPlayerMoving(p) {
    const targetX = p.x * tileSize;
    const targetY = p.y * tileSize;
    const dx = Math.abs(p.x_img - targetX);
    const dy = Math.abs(p.y_img - targetY);
    return dx > 0.1 || dy > 0.1;
}

function updateAnimations(deltaTime) {
    for (let p of players) {
        if (isPlayerMoving(p)) {
            p.animTime = (p.animTime || 0) + deltaTime;
            if (p.animTime >= 150) {
                p.animTime = 0;
                p.animFrame = animationStyle[(p.animFrame + 1) % framesPerAnim];
            }
        } else {
            p.animFrame = 1;
        }
    }
}

function draw() {
    ctx.clearRect(0, 0, canvas.width, canvas.height);

    const now = Date.now();
    const deltaTime = now - (draw.lastTime || now);
    draw.lastTime = now;

    updatePlayerPositions(deltaTime);
    updateAnimations(deltaTime);

    let myPlayer = players.find(p => p.id === myPlayerID);
    let cameraX = 0, cameraY = 0;

    // Escala baseada na quantidade de tiles visíveis
    const scaledTileW = canvas.width / tilesOnScreenX;
    const scaledTileH = canvas.height / tilesOnScreenY;

    const scaleX = scaledTileW / tileSize;
    const scaleY = scaledTileH / tileSize;

    if (myPlayer) {
        cameraX = myPlayer.x_img * scaleX + scaledTileW / 2 - canvas.width / 2;
        cameraY = myPlayer.y_img * scaleY + scaledTileH / 2 - canvas.height / 2;

        const maxCameraX = tileMapData.width * scaledTileW - canvas.width;
        const maxCameraY = tileMapData.height * scaledTileH - canvas.height;

        cameraX = Math.max(0, Math.min(cameraX, maxCameraX));
        cameraY = Math.max(0, Math.min(cameraY, maxCameraY));
    }

    drawMap(cameraX, cameraY);

    for (let p of players) {
        const dirIndex = directions.indexOf(p.direction || 'down');
        const charCol = p.characterIndex % charactersPerRow;
        const charRow = Math.floor(p.characterIndex / charactersPerRow);

        const sx = charCol * framesPerAnim * frameWidth + (p.animFrame % framesPerAnim) * frameWidth;
        const sy = charRow * directions.length * frameHeight + dirIndex * frameHeight;

        // Aplica escala aos personagens
        const destW = frameWidth * scaleX;
        const destH = frameHeight * scaleY;

        const dx = Math.round(p.x_img * scaleX - cameraX);
        const dy = Math.round(p.y_img * scaleY - cameraY - (destH - scaledTileH));

        ctx.drawImage(
            spriteSheet,
            sx, sy, frameWidth, frameHeight,
            dx, dy, destW, destH
        );

        // Desenhar mensagens no balão
        if (p.chatMessages && p.chatMessages.length > 0) {
            const messages = p.chatMessages.slice(-3);
            let yOffset = 0;

            ctx.font = "16px 'PixelMplus'";
            ctx.textBaseline = "middle";
            ctx.textAlign = "center";

            for (let i = messages.length - 1; i >= 0; i--) {
                const msg = messages[i];
                const elapsed = now - msg.timestamp;

                if (elapsed >= 15000) continue;

                let opacity = 1;
                if (elapsed > 10000) {
                    opacity = 1 - ((elapsed - 10000) / 5000);
                }

                ctx.save();
                ctx.globalAlpha = opacity;

                const text = msg.text;
                const textWidth = ctx.measureText(text).width;

                const x = Math.round(dx + destW / 2);
                const y = Math.round(dy - 10 - yOffset);

                ctx.fillStyle = "rgba(0, 0, 0, 0.7)";
                ctx.beginPath();
                ctx.roundRect(x - textWidth / 2 - 6, y - 10, textWidth + 12, 20, 6);
                ctx.fill();

                ctx.fillStyle = "white";
                ctx.fillText(text, x, y);

                ctx.restore();

                yOffset += 20;
            }
        }
    }

    requestAnimationFrame(draw);
}

draw();