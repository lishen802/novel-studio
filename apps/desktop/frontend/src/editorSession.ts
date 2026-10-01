// 编辑会话串行执行保存和版本提交，防止迟到请求覆盖较新的输入。
export class EditorSession {
  body: string;
  revision: number;
  private persisted: string;
  private tail: Promise<unknown> = Promise.resolve();
  readonly id: string;
  private readonly save: (id: string, body: string, revision: number) => Promise<void>;
  constructor(id: string, body: string, revision: number,
    save: (id: string, body: string, revision: number) => Promise<void>) {
    this.id = id;
    this.save = save;
    this.body = body;
    this.persisted = body;
    this.revision = revision;
  }
  get dirty() { return this.body !== this.persisted; }
  private enqueue<T>(action: () => Promise<T>): Promise<T> {
    const result = this.tail.catch(() => undefined).then(action);
    this.tail = result;
    return result;
  }
  private async persist() {
    while (this.dirty) {
      const body = this.body;
      await this.save(this.id, body, this.revision);
      this.persisted = body;
    }
  }
  flush(): Promise<void> { return this.enqueue(() => this.persist()); }
  commit(saveVersion: (id: string, body: string, revision: number) => Promise<{ body: string; revision: number }>) {
    return this.enqueue(async () => {
      await this.persist();
      const result = await saveVersion(this.id, this.body, this.revision);
      this.revision = result.revision;
      this.persisted = result.body;
      await this.persist();
      return result;
    });
  }
}
