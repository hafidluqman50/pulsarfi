'use client';

interface IconProps {
  name: string;
  size?: number;
  stroke?: number;
  color?: string;
  style?: React.CSSProperties;
}

export function Icon({ name, size = 16, stroke = 1.5, color = "currentColor", style }: IconProps) {
  const s: React.CSSProperties = { width: size, height: size, color, ...(style || {}) };
  const p = { fill: "none" as const, stroke: "currentColor", strokeWidth: stroke, strokeLinecap: "square" as const, strokeLinejoin: "miter" as const };
  switch (name) {
    case "wallet":      return <svg viewBox="0 0 24 24" style={s}><path {...p} d="M3 7h15v11H3z"/><path {...p} d="M3 7l3-3h12v3"/><circle {...p} cx="15" cy="12.5" r="1.2"/></svg>;
    case "chevron-down":return <svg viewBox="0 0 24 24" style={s}><path {...p} d="M5 9l7 7 7-7"/></svg>;
    case "chevron-right":return <svg viewBox="0 0 24 24" style={s}><path {...p} d="M9 5l7 7-7 7"/></svg>;
    case "search":      return <svg viewBox="0 0 24 24" style={s}><circle {...p} cx="11" cy="11" r="6"/><path {...p} d="M20 20l-4-4"/></svg>;
    case "x":           return <svg viewBox="0 0 24 24" style={s}><path {...p} d="M5 5l14 14M19 5L5 19"/></svg>;
    case "swap":        return <svg viewBox="0 0 24 24" style={s}><path {...p} d="M7 4v14M3 14l4 4 4-4M17 20V6M13 10l4-4 4 4"/></svg>;
    case "check":       return <svg viewBox="0 0 24 24" style={s}><path {...p} d="M4 12l5 5L20 6"/></svg>;
    case "info":        return <svg viewBox="0 0 24 24" style={s}><circle {...p} cx="12" cy="12" r="9"/><path {...p} d="M12 11v6M12 7.5v.5"/></svg>;
    case "external":    return <svg viewBox="0 0 24 24" style={s}><path {...p} d="M14 4h6v6M20 4l-9 9M19 14v6H5V5h6"/></svg>;
    case "send":        return <svg viewBox="0 0 24 24" style={s}><path {...p} d="M4 20l16-8L4 4l3 8-3 8z"/></svg>;
    case "menu":        return <svg viewBox="0 0 24 24" style={s}><path {...p} d="M4 7h16M4 12h16M4 17h16"/></svg>;
    case "copy":        return <svg viewBox="0 0 24 24" style={s}><rect {...p} x="8" y="8" width="12" height="12"/><path {...p} d="M16 4H4v12h4"/></svg>;
    case "settings":    return <svg viewBox="0 0 24 24" style={s}><circle {...p} cx="12" cy="12" r="3"/><path {...p} d="M19.4 15a1.7 1.7 0 0 0 .3 1.8l.1.1a2 2 0 1 1-2.8 2.8l-.1-.1a1.7 1.7 0 0 0-1.8-.3 1.7 1.7 0 0 0-1 1.5V21a2 2 0 1 1-4 0v-.1a1.7 1.7 0 0 0-1.1-1.5 1.7 1.7 0 0 0-1.8.3l-.1.1a2 2 0 1 1-2.8-2.8l.1-.1a1.7 1.7 0 0 0 .3-1.8 1.7 1.7 0 0 0-1.5-1H3a2 2 0 1 1 0-4h.1a1.7 1.7 0 0 0 1.5-1.1 1.7 1.7 0 0 0-.3-1.8l-.1-.1a2 2 0 1 1 2.8-2.8l.1.1a1.7 1.7 0 0 0 1.8.3H9a1.7 1.7 0 0 0 1-1.5V3a2 2 0 1 1 4 0v.1a1.7 1.7 0 0 0 1 1.5 1.7 1.7 0 0 0 1.8-.3l.1-.1a2 2 0 1 1 2.8 2.8l-.1.1a1.7 1.7 0 0 0-.3 1.8V9a1.7 1.7 0 0 0 1.5 1H21a2 2 0 1 1 0 4h-.1a1.7 1.7 0 0 0-1.5 1z"/></svg>;
    case "arrow-up":    return <svg viewBox="0 0 24 24" style={s}><path {...p} d="M12 20V4M5 11l7-7 7 7"/></svg>;
    case "arrow-down":  return <svg viewBox="0 0 24 24" style={s}><path {...p} d="M12 4v16M5 13l7 7 7-7"/></svg>;
    case "globe":       return <svg viewBox="0 0 24 24" style={s}><circle {...p} cx="12" cy="12" r="9"/><path {...p} d="M3 12h18M12 3a13 13 0 0 1 0 18M12 3a13 13 0 0 0 0 18"/></svg>;
    case "terminal":    return <svg viewBox="0 0 24 24" style={s}><rect {...p} x="3" y="4" width="18" height="16"/><path {...p} d="M7 9l3 3-3 3M12 16h5"/></svg>;
    case "play":        return <svg viewBox="0 0 24 24" style={s}><path {...p} d="M7 4l13 8L7 20V4z"/></svg>;
    case "loader":      return <svg viewBox="0 0 24 24" style={{ ...s, animation: 'spin .8s linear infinite' }}><path {...p} d="M12 3v4M12 17v4M3 12h4M17 12h4M5.6 5.6l2.8 2.8M15.6 15.6l2.8 2.8M18.4 5.6l-2.8 2.8M8.4 15.6l-2.8 2.8"/></svg>;
    default: return null;
  }
}
