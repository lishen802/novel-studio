import { BookOpenText, FolderOpen, Plus } from "lucide-react";
import "./styles.css";

export function App() {
  return (
    <main className="app-shell">
      <div className="app-shell__glow app-shell__glow--left" />
      <div className="app-shell__glow app-shell__glow--right" />

      <section className="welcome-card" aria-labelledby="product-title">
        <div className="product-mark" aria-hidden="true">
          <BookOpenText size={30} strokeWidth={1.65} />
        </div>
        <p className="eyebrow">长篇小说智能创作工作台</p>
        <h1 id="product-title">织境 Story Studio</h1>
        <p className="welcome-card__description">
          从故事工程开始，让智能体参与规划、写作与审阅，同时由作者掌握最终决定。
        </p>
        <div className="welcome-actions">
          <button type="button" className="primary-action">
            <Plus size={18} aria-hidden="true" />
            创建项目
          </button>
          <button type="button" className="secondary-action">
            <FolderOpen size={18} aria-hidden="true" />
            打开项目
          </button>
        </div>
        <p className="local-first-note">作品默认保存在本地，未经确认不会覆盖正文。</p>
      </section>
    </main>
  );
}
