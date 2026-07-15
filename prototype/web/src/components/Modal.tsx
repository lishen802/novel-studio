import type { ReactNode } from 'react'
import { X } from 'lucide-react'

export function Modal({ title, open, onClose, children, wide = false }: { title: string; open: boolean; onClose: () => void; children: ReactNode; wide?: boolean }) {
  if (!open) return null
  return (
    <div className="modal-backdrop" role="presentation" onMouseDown={(event) => event.target === event.currentTarget && onClose()}>
      <section className={`modal-card ${wide ? 'modal-wide' : ''}`} role="dialog" aria-modal="true" aria-label={title}>
        <header className="modal-header"><div><span className="eyebrow">织境工作台</span><h2>{title}</h2></div><button className="icon-button" aria-label="关闭" onClick={onClose}><X size={18} /></button></header>
        {children}
      </section>
    </div>
  )
}
