'use client';

import { useEffect, useRef, type PointerEvent } from 'react';
import type * as THREE from 'three';
import { BUILDINGS, INK, PEOPLE, SKIN_TONES, type PersonData, type TownStack } from './townData';

type ThreeModule = typeof THREE;

const BOARD_WIDTH = 560;
const BOARD_DEPTH = 400;
const COIN_PITCH = 11;
const CAMERA_DISTANCE = 1500;
const CAMERA_ELEVATION = (42 * Math.PI) / 180;
const START_ANGLE = 36;
const AUTO_ROTATE_STEP = 0.08;
const DRAG_FACTOR = 0.4;
const MOBILE_BREAKPOINT = 720;
const LIGHT_DIRECTION: [number, number, number] = [-0.5, 1, 0.65];
const SANS_FONT = 'var(--font-sans)';

let threeModule: Promise<ThreeModule> | null = null;
const loadThree = () => (threeModule ??= import('three'));

interface TownScene3DProps {
  stacks: TownStack[];
  autoRotate: boolean;
  reducedMotion: boolean;
  onUnsupported: () => void;
}

interface Motion {
  angle: number;
  dragging: boolean;
  lastX: number;
  velocity: number;
}

interface Anchored {
  position: THREE.Vector3;
  element: HTMLElement;
}

interface FloatTag extends Anchored {
  startedAt: number;
}

interface PersonRig {
  person: PersonData;
  sprite: THREE.Sprite;
  shadow: THREE.Mesh;
  bubble: HTMLElement;
  bubbleText: HTMLElement;
  head: THREE.Vector3;
  lastCycle: number;
}

interface TownOptions {
  three: ThreeModule;
  wrapper: HTMLElement;
  host: HTMLElement;
  overlay: HTMLElement;
  stacks: TownStack[];
  motion: Motion;
  getAutoRotate: () => boolean;
  getReducedMotion: () => boolean;
}

function createOverlayElement(overlay: HTMLElement, css: string, text?: string): HTMLDivElement {
  const element = document.createElement('div');
  element.style.cssText = `position:absolute;left:0;top:0;will-change:transform;pointer-events:none;${css}`;
  if (text !== undefined) element.textContent = text;
  overlay.appendChild(element);
  return element;
}

function boardToWorld(x: number, y: number): [number, number] {
  return [x - BOARD_WIDTH / 2, y - BOARD_DEPTH / 2];
}

function personSvg(person: PersonData, skin: string): string {
  const eyes = `<circle cx="15.2" cy="13.5" r=".9" fill="${INK}"/><circle cx="18.2" cy="13.5" r=".9" fill="${INK}"/>`;
  const body = person.kind === 'idx'
    ? `<path d="M4 52 L5 27 Q14 20 23 27 L24 52 Z" fill="${INK}"/><path d="M10.5 22.5 L14 31 L17.5 22.5 Z" fill="#fff"/><path d="M13.3 24 L14.7 24 L15.2 32 L14 34 L12.8 32 Z" fill="#c8102e"/><rect x="17" y="32" width="4" height="5" fill="#fff" stroke="#c8102e" stroke-width=".8"/><circle cx="14" cy="13" r="7" fill="${skin}" stroke="${INK}"/><path d="M7 12 Q8 5 14 5.5 Q20 5 21 11 Q17 8.5 12 9.5 Q9 10 7 12 Z" fill="${INK}"/>`
    : `<path d="M3 52 L4 28 Q14 19 24 28 L25 52 Z" fill="${person.color}" stroke="${INK}"/><path d="M6 18 Q6 4 14 4 Q22 4 22 18 Z" fill="${person.color}" stroke="${INK}"/><path d="M9 38 L19 38 L18 44 L10 44 Z" fill="rgba(0,0,0,.18)"/><circle cx="14" cy="14" r="6.2" fill="${skin}" stroke="${INK}"/><rect x="19" y="30" width="6" height="9" rx="1" fill="${INK}" stroke="#fbfaf7" stroke-width=".6"/>`;
  return `<svg xmlns="http://www.w3.org/2000/svg" width="112" height="208" viewBox="0 0 28 52">${body}${eyes}</svg>`;
}

function createTown(options: TownOptions): () => void {
  const { three, wrapper, host, overlay, stacks, motion, getAutoRotate, getReducedMotion } = options;
  const renderer = new three.WebGLRenderer({ antialias: true, alpha: true });
  renderer.domElement.style.display = 'block';
  host.appendChild(renderer.domElement);

  const scene = new three.Scene();
  const camera = new three.OrthographicCamera(-1, 1, 1, -1, 1, 5000);
  const cameraTarget = new three.Vector3(0, 40, 10);
  const lineMaterial = new three.LineBasicMaterial({ color: INK });
  const lightDirection = new three.Vector3(...LIGHT_DIRECTION).normalize();
  const disposables: { dispose: () => void }[] = [renderer];
  const track = <Item extends { dispose: () => void }>(item: Item): Item => {
    disposables.push(item);
    return item;
  };
  track(lineMaterial);

  const shade = (geometry: THREE.BufferGeometry) => {
    const normals = geometry.attributes.normal;
    const colors = new Float32Array(normals.count * 3);
    for (let index = 0; index < normals.count; index++) {
      const normalY = normals.getY(index);
      const lit = normals.getX(index) * lightDirection.x + normalY * lightDirection.y + normals.getZ(index) * lightDirection.z;
      const value = normalY > 0.9 ? 1 : normalY < -0.9 ? 0.5 : 0.56 + 0.36 * Math.max(0, lit);
      colors[index * 3] = colors[index * 3 + 1] = colors[index * 3 + 2] = value;
    }
    geometry.setAttribute('color', new three.BufferAttribute(colors, 3));
    return geometry;
  };

  const flatMaterial = (parameters: THREE.MeshBasicMaterialParameters) => track(new three.MeshBasicMaterial({
    vertexColors: true, polygonOffset: true, polygonOffsetFactor: 1, polygonOffsetUnits: 1, ...parameters,
  }));

  const canvasTexture = (width: number, height: number, draw: (context: CanvasRenderingContext2D) => void, repeat = false) => {
    const canvas = document.createElement('canvas');
    canvas.width = width;
    canvas.height = height;
    const context = canvas.getContext('2d');
    if (context) draw(context);
    const texture = track(new three.CanvasTexture(canvas));
    texture.colorSpace = three.SRGBColorSpace;
    texture.anisotropy = 4;
    if (repeat) texture.wrapS = texture.wrapT = three.RepeatWrapping;
    return texture;
  };

  const outlined = (mesh: THREE.Mesh, geometry: THREE.BufferGeometry) => {
    mesh.add(new three.LineSegments(track(new three.EdgesGeometry(geometry, 1)), lineMaterial));
    return mesh;
  };

  const groundTexture = canvasTexture(1160, 840, (context) => {
    const width = 1160;
    const height = 840;
    const toPixel = (value: number) => (value + 10) * 2;
    context.fillStyle = '#f8f6f1';
    context.fillRect(0, 0, width, height);
    context.fillStyle = 'rgba(22,17,14,.05)';
    for (let speck = 0; speck < 1400; speck++) context.fillRect(Math.random() * width, Math.random() * height, 2, 2);
    const road = (top: number, thickness: number) => {
      context.fillStyle = '#e3ddd2';
      context.fillRect(0, toPixel(top), width, thickness * 2);
      context.strokeStyle = '#a39988';
      context.lineWidth = 2;
      context.setLineDash([]);
      context.strokeRect(-4, toPixel(top), width + 8, thickness * 2);
      context.strokeStyle = '#fbfaf7';
      context.lineWidth = 3;
      context.setLineDash([18, 14]);
      context.beginPath();
      context.moveTo(0, toPixel(top + thickness / 2));
      context.lineTo(width, toPixel(top + thickness / 2));
      context.stroke();
    };
    road(318, 36);
    road(44, 28);
    context.setLineDash([]);
    context.fillStyle = '#fbfaf7';
    for (let stripe = 0; stripe < 6; stripe++) {
      context.fillRect(toPixel(250) + stripe * 14, toPixel(320), 8, 64);
      context.fillRect(toPixel(250) + stripe * 14, toPixel(46), 8, 52);
    }
    context.fillStyle = '#efebe3';
    context.beginPath();
    context.ellipse(toPixel(275), toPixel(200), 330, 260, 0, 0, Math.PI * 2);
    context.fill();
    context.strokeStyle = '#bcb2a3';
    context.lineWidth = 2;
    context.setLineDash([10, 8]);
    context.beginPath();
    context.ellipse(toPixel(275), toPixel(200), 330, 260, 0, 0, Math.PI * 2);
    context.stroke();
    context.setLineDash([]);
    context.strokeStyle = '#d6cfc1';
    context.beginPath();
    context.ellipse(toPixel(275), toPixel(200), 300, 232, 0, 0, Math.PI * 2);
    context.stroke();
  });

  const addSlab = (width: number, depth: number, centerY: number, thickness: number, topMaterial: THREE.Material, sideColor: string, withOutline: boolean) => {
    const geometry = shade(track(new three.BoxGeometry(width, thickness, depth)));
    const sideMaterial = flatMaterial({ color: sideColor });
    const mesh = new three.Mesh(geometry, [sideMaterial, sideMaterial, topMaterial, sideMaterial, sideMaterial, sideMaterial]);
    mesh.position.y = centerY;
    scene.add(withOutline ? outlined(mesh, geometry) : mesh);
  };
  addSlab(580, 420, -7, 14, flatMaterial({ map: groundTexture, vertexColors: false }), '#e3ddd2', true);
  addSlab(604, 444, -19, 10, flatMaterial({ color: '#d6cfc1' }), '#c9bfae', true);

  const signs: Anchored[] = [];
  const windowTextures = new Map<string, THREE.CanvasTexture>();
  const windowTexture = (faceColor: string, windowColor: string) => {
    const key = faceColor + windowColor;
    const cached = windowTextures.get(key);
    if (cached) return cached;
    const texture = canvasTexture(32, 32, (context) => {
      context.fillStyle = faceColor;
      context.fillRect(0, 0, 32, 32);
      context.fillStyle = windowColor;
      context.fillRect(7, 9, 18, 13);
      context.fillStyle = 'rgba(22,17,14,.25)';
      context.fillRect(7, 21, 18, 1);
    }, true);
    windowTextures.set(key, texture);
    return texture;
  };

  BUILDINGS.forEach((building) => {
    const geometry = shade(track(new three.BoxGeometry(building.width, building.height, building.depth)));
    const uv = geometry.attributes.uv;
    for (let index = 0; index < 24; index++) {
      if (index >= 8 && index < 16) continue;
      const horizontalRepeat = (index < 8 ? building.depth : building.width) / 14;
      uv.setXY(index, uv.getX(index) * horizontalRepeat, (uv.getY(index) * building.height) / 16);
    }
    const wallMaterial = flatMaterial({ map: windowTexture(building.faceColor, building.windowColor) });
    const roofMaterial = flatMaterial({ color: building.roofColor });
    const mesh = new three.Mesh(geometry, [wallMaterial, wallMaterial, roofMaterial, roofMaterial, wallMaterial, wallMaterial]);
    const [centerX, centerZ] = boardToWorld(building.x + building.width / 2, building.y + building.depth / 2);
    mesh.position.set(centerX, building.height / 2, centerZ);
    scene.add(outlined(mesh, geometry));
    if (building.sign) {
      const element = createOverlayElement(
        overlay,
        `background:${building.sign.background};color:#fff;border:1px solid ${INK};padding:4px 7px;font:700 10px ${SANS_FONT};letter-spacing:.14em;white-space:nowrap`,
        building.sign.text,
      );
      signs.push({ position: new three.Vector3(centerX, building.height + 14, centerZ), element });
    }
  });

  const shadowTexture = canvasTexture(64, 64, (context) => {
    const gradient = context.createRadialGradient(32, 32, 0, 32, 32, 32);
    gradient.addColorStop(0, 'rgba(22,17,14,.26)');
    gradient.addColorStop(1, 'rgba(22,17,14,0)');
    context.fillStyle = gradient;
    context.fillRect(0, 0, 64, 64);
  });
  const shadowMaterial = track(new three.MeshBasicMaterial({ map: shadowTexture, transparent: true, depthWrite: false }));
  const shadowGeometry = track(new three.PlaneGeometry(1, 1)).rotateX(-Math.PI / 2);
  const addShadow = (x: number, z: number, width: number, depth: number) => {
    const mesh = new three.Mesh(shadowGeometry, shadowMaterial);
    mesh.scale.set(width, 1, depth);
    mesh.position.set(x, 0.4, z);
    scene.add(mesh);
    return mesh;
  };

  const coinGeometry = shade(track(new three.CylinderGeometry(44, 44, 10, 56)));
  const coinEdges = track(new three.EdgesGeometry(coinGeometry, 30));
  const coinTopMaterial = flatMaterial({ color: '#ffffff' });
  const coinBottomMaterial = flatMaterial({ color: '#a39988' });
  const textureLoader = new three.TextureLoader();
  const stackLabels: Anchored[] = [];
  const floatTags: FloatTag[] = [];

  stacks.forEach((stack) => {
    const [x, z] = boardToWorld(stack.slotX, stack.slotY);
    addShadow(x, z, 160, 160);
    const sideMaterial = flatMaterial({
      map: canvasTexture(4, 32, (context) => {
        context.fillStyle = '#d6cfc1';
        context.fillRect(0, 0, 4, 32);
        context.fillStyle = stack.edgeColor;
        context.fillRect(0, 11, 4, 6);
        context.fillStyle = '#a39988';
        context.fillRect(0, 25, 4, 7);
      }),
    });
    for (let coin = 0; coin < stack.coinCount; coin++) {
      const mesh = new three.Mesh(coinGeometry, [sideMaterial, coinTopMaterial, coinBottomMaterial]);
      mesh.position.set(x, coin * COIN_PITCH + 5, z);
      mesh.add(new three.LineSegments(coinEdges, lineMaterial));
      scene.add(mesh);
    }
    const topY = stack.coinCount * COIN_PITCH - 0.6;
    const ring = new three.Mesh(track(new three.RingGeometry(35, 38, 56)).rotateX(-Math.PI / 2), track(new three.MeshBasicMaterial({ color: stack.edgeColor })));
    ring.position.set(x, topY + 0.9, z);
    scene.add(ring);
    const logo = textureLoader.load(stack.iconUrl);
    logo.colorSpace = three.SRGBColorSpace;
    track(logo);
    const logoMesh = new three.Mesh(
      track(new three.PlaneGeometry(46, 46)).rotateX(-Math.PI / 2),
      track(new three.MeshBasicMaterial({ map: logo, transparent: true, alphaTest: 0.05 })),
    );
    logoMesh.position.set(x, topY + 1, z);
    scene.add(logoMesh);

    const label = createOverlayElement(overlay, `background:#fff;border:1px solid ${INK};padding:5px 8px;white-space:nowrap;box-shadow:0 3px 0 -1px #fbfaf7,0 4px 0 -1px ${INK}`);
    const labelTicker = document.createElement('span');
    labelTicker.style.cssText = `font:700 11px ${SANS_FONT};letter-spacing:.04em`;
    labelTicker.textContent = stack.ticker;
    const labelChange = document.createElement('span');
    labelChange.className = 'mono';
    labelChange.style.cssText = `font-size:11px;font-weight:500;margin-left:4px;color:${stack.changeColor}`;
    labelChange.textContent = stack.changeLabel;
    label.append(labelTicker, labelChange);
    stackLabels.push({ position: new three.Vector3(x, stack.stackHeight + 40, z), element: label });

    const tag = createOverlayElement(overlay, 'opacity:0;background:#1f7a4b;color:#fff;padding:3px 6px;font-size:10px;font-weight:700;white-space:nowrap', '+1 buy');
    tag.className = 'mono';
    floatTags.push({ position: new three.Vector3(x, stack.stackHeight + 70, z), element: tag, startedAt: -9 });
  });

  const rigs: PersonRig[] = PEOPLE.map((person, index) => {
    const canvas = document.createElement('canvas');
    canvas.width = 128;
    canvas.height = 224;
    const texture = track(new three.CanvasTexture(canvas));
    texture.colorSpace = three.SRGBColorSpace;
    const image = new Image();
    image.onload = () => {
      const outline = document.createElement('canvas');
      outline.width = 128;
      outline.height = 224;
      const outlineContext = outline.getContext('2d');
      const context = canvas.getContext('2d');
      if (!outlineContext || !context) return;
      for (let step = 0; step < 12; step++) {
        outlineContext.drawImage(image, 8 + Math.cos((step / 12) * Math.PI * 2) * 6, 8 + Math.sin((step / 12) * Math.PI * 2) * 6);
      }
      outlineContext.globalCompositeOperation = 'source-in';
      outlineContext.fillStyle = '#fbfaf7';
      outlineContext.fillRect(0, 0, 128, 224);
      context.drawImage(outline, 0, 0);
      context.drawImage(image, 8, 8);
      texture.needsUpdate = true;
    };
    image.src = `data:image/svg+xml;charset=utf-8,${encodeURIComponent(personSvg(person, SKIN_TONES[index % SKIN_TONES.length]))}`;

    const sprite = new three.Sprite(track(new three.SpriteMaterial({ map: texture, alphaTest: 0.4 })));
    sprite.center.set(0.5, 8 / 224);
    sprite.scale.set(32, 56, 1);
    scene.add(sprite);
    const shadow = addShadow(0, 0, 30, 14);

    const isIdx = person.kind === 'idx';
    const textColor = isIdx || person.color === '#e7b416' ? INK : person.color;
    const bubble = createOverlayElement(
      overlay,
      `opacity:0;transition:opacity .2s;background:#fff;border:1px solid ${INK};padding:3px 7px;white-space:nowrap;font-size:10px;font-weight:600;color:${textColor};${isIdx ? `font-family:${SANS_FONT}` : ''}`,
    );
    if (!isIdx) bubble.className = 'mono';
    const bubbleText = document.createElement('span');
    const bubbleArrow = document.createElement('span');
    bubbleArrow.style.cssText = `position:absolute;left:50%;bottom:-5px;width:8px;height:8px;background:#fff;border-right:1px solid ${INK};border-bottom:1px solid ${INK};transform:translateX(-50%) rotate(45deg)`;
    bubble.append(bubbleText, bubbleArrow);
    return { person, sprite, shadow, bubble, bubbleText, head: new three.Vector3(), lastCycle: -1 };
  });

  let width = 1;
  let height = 1;
  const resize = () => {
    width = wrapper.clientWidth || 1;
    height = wrapper.clientHeight || 1;
    renderer.setPixelRatio(Math.min(window.devicePixelRatio || 1, width < MOBILE_BREAKPOINT ? 1.5 : 2));
    renderer.setSize(width, height);
    const zoom = Math.min(width / 600, height / 500) * (width < MOBILE_BREAKPOINT ? 1.08 : 1);
    const halfWidth = width / 2 / zoom;
    const halfHeight = height / 2 / zoom;
    camera.left = -halfWidth;
    camera.right = halfWidth;
    camera.top = halfHeight;
    camera.bottom = -halfHeight;
    camera.updateProjectionMatrix();
  };
  const resizeObserver = new ResizeObserver(resize);
  resizeObserver.observe(wrapper);
  resize();

  const projected = new three.Vector3();
  const cameraRight = new three.Vector3();
  const place = (element: HTMLElement, position: THREE.Vector3, offsetY = 0) => {
    projected.copy(position).project(camera);
    const x = (projected.x * 0.5 + 0.5) * width;
    const y = (-projected.y * 0.5 + 0.5) * height + offsetY;
    element.style.transform = `translate3d(${x.toFixed(1)}px,${y.toFixed(1)}px,0) translate(-50%,-100%)`;
  };

  let frameId = 0;
  let isVisible = true;
  let disposed = false;

  const tick = () => {
    if (disposed || !isVisible || document.hidden) {
      frameId = 0;
      return;
    }
    const reducedMotion = getReducedMotion();
    const time = reducedMotion ? 0 : performance.now() / 1000;
    if (!motion.dragging) {
      motion.angle += (getAutoRotate() ? AUTO_ROTATE_STEP : 0) + motion.velocity;
      motion.velocity *= 0.94;
    }
    const angleRadians = (motion.angle * Math.PI) / 180;
    camera.position.set(
      cameraTarget.x + Math.sin(angleRadians) * Math.cos(CAMERA_ELEVATION) * CAMERA_DISTANCE,
      cameraTarget.y + Math.sin(CAMERA_ELEVATION) * CAMERA_DISTANCE,
      cameraTarget.z + Math.cos(angleRadians) * Math.cos(CAMERA_ELEVATION) * CAMERA_DISTANCE,
    );
    camera.lookAt(cameraTarget);
    camera.updateMatrixWorld();
    cameraRight.setFromMatrixColumn(camera.matrixWorld, 0);

    rigs.forEach((rig) => {
      const { person } = rig;
      const stackIndex = (person.stackIndex ?? 0) % stacks.length;
      let x = person.x ?? 0;
      let y = person.y ?? 0;
      let direction = Math.sin(time * 1.6 + person.phase * 2) > 0.15 ? 1 : -1;
      let bob = 0;
      let text = '';
      let show = false;
      if (person.action === 'walk' && person.path) {
        const [[startX, startY], [endX, endY]] = person.path;
        const length = Math.hypot(endX - startX, endY - startY);
        const travelled = ((time + person.phase) * (person.speed ?? 20)) % (2 * length);
        const forward = travelled < length;
        const progress = forward ? travelled / length : 2 - travelled / length;
        x = startX + (endX - startX) * progress;
        y = startY + (endY - startY) * progress;
        const velocityX = (endX - startX) * (forward ? 1 : -1);
        const velocityZ = (endY - startY) * (forward ? 1 : -1);
        direction = velocityX * cameraRight.x + velocityZ * cameraRight.z >= 0 ? 1 : -1;
        bob = reducedMotion ? 0 : Math.abs(Math.sin(time * 9 + person.phase)) * 2.5;
        show = (time + person.phase) % 9 < 2.5;
        text = person.say?.[0] ?? '';
      } else if (person.action === 'count') {
        const counter = (time + person.phase) % 7;
        show = counter < 4.5;
        text = `${stacks[stackIndex].ticker.slice(0, 4)} · ${Math.floor(counter * 2.4) + 1}…`;
      } else {
        const lines = person.say ?? [];
        const cycle = Math.floor((time + person.phase) / 5);
        const within = (time + person.phase) % 5;
        show = within < 2.2;
        text = lines[cycle % lines.length] ?? '';
        if (show && cycle !== rig.lastCycle) {
          rig.lastCycle = cycle;
          if (/BUY|SWAP/.test(text)) {
            const tag = floatTags[stackIndex];
            tag.startedAt = time;
            tag.element.textContent = text.startsWith('SWAP') ? '⇄ swap' : '+1 buy';
          }
        }
        bob = show && within < 0.4 ? Math.sin((within / 0.4) * Math.PI) * 4 : 0;
      }
      if (reducedMotion) show = false;
      const [worldX, worldZ] = boardToWorld(x, y);
      rig.sprite.position.set(worldX, bob, worldZ);
      rig.sprite.scale.x = 32 * direction;
      rig.shadow.position.set(worldX, 0.5, worldZ);
      rig.bubble.style.opacity = show ? '1' : '0';
      if (rig.bubbleText.textContent !== text) rig.bubbleText.textContent = text;
      if (show) place(rig.bubble, rig.head.set(worldX, bob + 56, worldZ), -8);
    });

    signs.forEach(sign => place(sign.element, sign.position));
    stackLabels.forEach(label => place(label.element, label.position));
    floatTags.forEach((tag) => {
      const elapsed = time - tag.startedAt;
      const isOn = !reducedMotion && elapsed >= 0 && elapsed < 1.6;
      tag.element.style.opacity = isOn ? String(1 - elapsed / 1.6) : '0';
      if (isOn) place(tag.element, tag.position, -elapsed * 30);
    });
    renderer.render(scene, camera);
    frameId = requestAnimationFrame(tick);
  };

  const start = () => {
    if (!frameId && !disposed && isVisible && !document.hidden) frameId = requestAnimationFrame(tick);
  };
  const intersectionObserver = new IntersectionObserver(([entry]) => {
    isVisible = entry.isIntersecting;
    start();
  }, { rootMargin: '100px' });
  intersectionObserver.observe(wrapper);
  document.addEventListener('visibilitychange', start);
  start();

  return () => {
    disposed = true;
    cancelAnimationFrame(frameId);
    document.removeEventListener('visibilitychange', start);
    intersectionObserver.disconnect();
    resizeObserver.disconnect();
    disposables.forEach(item => item.dispose());
    renderer.forceContextLoss();
    renderer.domElement.remove();
    overlay.replaceChildren();
  };
}

export function TownScene3D({ stacks, autoRotate, reducedMotion, onUnsupported }: TownScene3DProps) {
  const wrapperRef = useRef<HTMLDivElement>(null);
  const hostRef = useRef<HTMLDivElement>(null);
  const overlayRef = useRef<HTMLDivElement>(null);
  const motion = useRef<Motion>({ angle: START_ANGLE, dragging: false, lastX: 0, velocity: 0 });
  const autoRotateRef = useRef(autoRotate);
  const reducedMotionRef = useRef(reducedMotion);
  const onUnsupportedRef = useRef(onUnsupported);

  useEffect(() => {
    autoRotateRef.current = autoRotate;
    reducedMotionRef.current = reducedMotion;
    onUnsupportedRef.current = onUnsupported;
  }, [autoRotate, reducedMotion, onUnsupported]);

  useEffect(() => {
    const wrapper = wrapperRef.current;
    const host = hostRef.current;
    const overlay = overlayRef.current;
    if (!wrapper || !host || !overlay) return;
    let cancelled = false;
    let dispose = () => {};
    loadThree()
      .then((three) => {
        if (cancelled) return;
        dispose = createTown({
          three, wrapper, host, overlay, stacks, motion: motion.current,
          getAutoRotate: () => autoRotateRef.current,
          getReducedMotion: () => reducedMotionRef.current,
        });
      })
      .catch(() => {
        if (!cancelled) onUnsupportedRef.current();
      });
    return () => {
      cancelled = true;
      dispose();
    };
  }, [stacks]);

  const handlePointerDown = (event: PointerEvent<HTMLDivElement>) => {
    motion.current.dragging = true;
    motion.current.lastX = event.clientX;
    event.currentTarget.setPointerCapture(event.pointerId);
    event.currentTarget.style.cursor = 'grabbing';
  };

  const handlePointerMove = (event: PointerEvent<HTMLDivElement>) => {
    const state = motion.current;
    if (!state.dragging) return;
    const deltaX = event.clientX - state.lastX;
    state.lastX = event.clientX;
    state.angle -= deltaX * DRAG_FACTOR;
    state.velocity = -deltaX * 0.06;
  };

  const handlePointerUp = (event: PointerEvent<HTMLDivElement>) => {
    motion.current.dragging = false;
    event.currentTarget.style.cursor = 'grab';
  };

  return (
    <div
      ref={wrapperRef}
      onPointerDown={handlePointerDown}
      onPointerMove={handlePointerMove}
      onPointerUp={handlePointerUp}
      onPointerCancel={handlePointerUp}
      style={{ position: 'relative', width: '100%', height: 'clamp(420px, 100vw, 640px)', overflow: 'hidden', cursor: 'grab', touchAction: 'pan-y', userSelect: 'none' }}
    >
      <div ref={hostRef} style={{ position: 'absolute', inset: 0 }} />
      <div ref={overlayRef} style={{ position: 'absolute', inset: 0, pointerEvents: 'none', overflow: 'hidden' }} />
    </div>
  );
}
