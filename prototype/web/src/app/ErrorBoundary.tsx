import { Component, type ErrorInfo, type ReactNode } from 'react'
import { AlertTriangle, Download, RefreshCw, RotateCcw } from 'lucide-react'
import { downloadText } from '../services/exportService'
import { STORAGE_KEY } from '../store/persistence'

export class ErrorBoundary extends Component<{ children: ReactNode }, { failed: boolean; message: string }> {
  state = { failed: false, message: '' }
  static getDerivedStateFromError(error: Error) { return { failed: true, message: error.message } }
  componentDidCatch(error: Error, info: ErrorInfo) { console.error('Novel Studio render failure', error, info) }
  render() {
    if (!this.state.failed) return this.props.children
    return <main className="fatal-error"><AlertTriangle size={48}/><span className="eyebrow">RECOVERY MODE</span><h1>工作区遇到错误</h1><p>原始项目数据仍保存在浏览器中。你可以重试、导出恢复数据，或重置演示项目。</p><code>{this.state.message}</code><div className="toolbar"><button className="primary" onClick={()=>this.setState({failed:false,message:''})}><RefreshCw size={16}/>重试渲染</button><button className="secondary" onClick={()=>downloadText('novel-studio-recovery.json',localStorage.getItem(STORAGE_KEY)??'{}','application/json')}><Download size={16}/>导出恢复数据</button><button className="secondary" onClick={()=>{localStorage.removeItem(STORAGE_KEY);window.location.reload()}}><RotateCcw size={16}/>重置演示数据</button></div></main>
  }
}
