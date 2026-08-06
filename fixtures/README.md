# Fixtures

保存无敏感信息、无版权风险的合成测试规格、导入文本和模型响应样本。

## 规则

- 不保存真实作者稿件、真实项目数据库或完整第三方小说；
- 不保存 `.zjproject`、`.db`、`.db-wal`、`.db-shm`、API Key、私钥或签名文件；
- 长篇测试优先保存固定种子、生成规格和黄金查询，在测试临时目录生成正文；
- Recorded Provider 响应必须脱敏，不包含请求凭据、用户正文或 Provider 私有身份信息；
- 每个 fixture 在对应测试或目录说明中写明用途和合成方式。

本地检查：

```bash
node --test scripts/ci/tracked-artifact-policy.test.mjs
node scripts/ci/tracked-artifact-policy.mjs
```
