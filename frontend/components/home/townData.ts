export const INK = '#16110e';
export const SKIN_TONES = ['#f1d9c0', '#d9a77c', '#b9835a', '#e8c39e'] as const;

export interface BuildingData {
  id: string;
  x: number;
  y: number;
  width: number;
  depth: number;
  height: number;
  faceColor: string;
  windowColor: string;
  roofColor: string;
  sign?: { text: string; background: string };
}

export const BUILDINGS: BuildingData[] = [
  { id: 'idx', x: 16, y: 96, width: 72, depth: 70, height: 128, faceColor: '#fbfaf7', windowColor: '#c9d3df', roofColor: INK, sign: { text: 'IDX', background: '#c8102e' } },
  { id: 'cus', x: 452, y: 86, width: 92, depth: 60, height: 84, faceColor: '#f3efe7', windowColor: '#d6cfc1', roofColor: '#e3ddd2', sign: { text: 'CUSTODIAN', background: INK } },
  { id: 'arb', x: 480, y: 190, width: 64, depth: 64, height: 58, faceColor: '#1f4d8a', windowColor: '#9fb8d8', roofColor: INK, sign: { text: 'ARBITRUM', background: '#1f4d8a' } },
  { id: 'ofc', x: 16, y: 196, width: 60, depth: 96, height: 74, faceColor: '#efebe3', windowColor: '#b9b0a2', roofColor: '#d6cfc1' },
  { id: 'shp1', x: 16, y: 362, width: 120, depth: 34, height: 30, faceColor: '#fbfaf7', windowColor: '#e9b8bf', roofColor: '#c8102e' },
  { id: 'shp2', x: 424, y: 362, width: 120, depth: 34, height: 38, faceColor: '#f3efe7', windowColor: '#d6cfc1', roofColor: INK },
  { id: 'tw', x: 16, y: 6, width: 110, depth: 32, height: 52, faceColor: '#e3ddd2', windowColor: '#fbfaf7', roofColor: INK },
  { id: 'tw2', x: 440, y: 6, width: 104, depth: 32, height: 64, faceColor: '#fbfaf7', windowColor: '#c9d3df', roofColor: '#e3ddd2' },
];

export type PersonAction = 'count' | 'trade' | 'walk';

export interface PersonData {
  kind: 'idx' | 'web3';
  color?: string;
  x?: number;
  y?: number;
  action: PersonAction;
  stackIndex?: number;
  phase: number;
  say?: string[];
  path?: readonly [readonly [number, number], readonly [number, number]];
  speed?: number;
}

export const PEOPLE: PersonData[] = [
  { kind: 'idx', x: 255, y: 82, action: 'count', stackIndex: 1, phase: 0 },
  { kind: 'idx', x: 440, y: 178, action: 'count', stackIndex: 3, phase: 1.3 },
  { kind: 'idx', x: 108, y: 176, action: 'trade', stackIndex: 0, phase: 0.4, say: ['BUY BBCAP', 'Backed 1:1?', 'BUY BBCAP'] },
  { kind: 'idx', action: 'walk', path: [[120, 336], [450, 336]], speed: 26, phase: 0, say: ['Checking custody…'] },
  { kind: 'web3', color: '#1f4d8a', x: 330, y: 322, action: 'trade', stackIndex: 2, phase: 2.1, say: ['SWAP → IDRX', 'on-chain ✓', 'SWAP → BRPT'] },
  { kind: 'web3', color: '#c8102e', x: 445, y: 104, action: 'trade', stackIndex: 1, phase: 3.2, say: ['gm', 'BUY BUMIP', 'LFG'] },
  { kind: 'web3', color: '#e7b416', x: 498, y: 272, action: 'count', stackIndex: 3, phase: 0.8 },
  { kind: 'web3', color: '#2a231e', action: 'walk', path: [[140, 58], [420, 58]], speed: 20, phase: 4, say: ['wen BBRI?'] },
];

export interface TownStack {
  ticker: string;
  coinCount: number;
  stackHeight: number;
  slotX: number;
  slotY: number;
  edgeColor: string;
  changeLabel: string;
  changeColor: string;
  iconUrl: string;
}
