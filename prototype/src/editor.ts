export interface CanvasPreset {
  id: string;
  label: string;
  bg: string;
  text: string;
}

export const canvasPresets: CanvasPreset[] = [
  { id: 'paper', label: '白纸', bg: '#ffffff', text: '#1f2937' },
  { id: 'cream', label: '羊皮', bg: '#f7f3e8', text: '#3d3a33' },
  { id: 'green', label: '护眼', bg: '#e8f5e8', text: '#2a3a2a' },
  { id: 'dark', label: '深夜', bg: '#1a1d24', text: '#e8e6e1' },
];

export type MarginSize = 'narrow' | 'medium' | 'wide';

export const defaultEditorSettings = {
  fontSize: 18,
  lineHeight: 1.9,
  canvasColorId: 'paper',
  marginSize: 'medium' as MarginSize,
  indent: true,
};
