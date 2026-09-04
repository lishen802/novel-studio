import { access, readFile } from "node:fs/promises";
import { constants } from "node:fs";
import path from "node:path";

const bindingsRoot = path.resolve("frontend/wailsjs/go");
const files = {
  apiDeclaration: path.join(bindingsRoot, "main", "DesktopAPI.d.ts"),
  apiRuntime: path.join(bindingsRoot, "main", "DesktopAPI.js"),
  models: path.join(bindingsRoot, "models.ts"),
};

for (const [name, file] of Object.entries(files)) {
  try {
    await access(file, constants.R_OK);
  } catch {
    throw new Error(
      `${name} is missing. Run \`wails generate module\` from apps/desktop first.`,
    );
  }
}

const [apiDeclaration, apiRuntime, models] = await Promise.all([
  readFile(files.apiDeclaration, "utf8"),
  readFile(files.apiRuntime, "utf8"),
  readFile(files.models, "utf8"),
]);

const expected = [
  [
    "OpenProject declaration",
    apiDeclaration,
    "OpenProject(arg1:main.ProjectOpenInput):Promise<main.ProjectOpenResultView>",
  ],
  [
    "CreateProjectRecoveryCopy declaration",
    apiDeclaration,
    "CreateProjectRecoveryCopy(arg1:main.RecoveryCopyInput):Promise<main.RecoveryCopyResultView>",
  ],
  ["OpenProject runtime binding", apiRuntime, "export function OpenProject(arg1)"],
  [
    "CreateProjectRecoveryCopy runtime binding",
    apiRuntime,
    "export function CreateProjectRecoveryCopy(arg1)",
  ],
  ["ProjectOpenInput model", models, "export class ProjectOpenInput"],
  ["ProjectOpenResultView model", models, "export class ProjectOpenResultView"],
  [
    "ProjectLeaseConflictView model",
    models,
    "export class ProjectLeaseConflictView",
  ],
  ["RecoveryCopyInput model", models, "export class RecoveryCopyInput"],
  [
    "RecoveryCopyResultView model",
    models,
    "export class RecoveryCopyResultView",
  ],
  ["ProjectView accessMode", models, "accessMode: string;"],
  [
    "CreationWorkbench declaration",
    apiDeclaration,
    "CreationWorkbench():Promise<creation.WorkbenchView>",
  ],
  [
    "RecoverCreation declaration",
    apiDeclaration,
    "RecoverCreation():Promise<creation.WorkbenchView>",
  ],
  [
    "ConfirmCreationGenerate declaration",
    apiDeclaration,
    "ConfirmCreationGenerate(arg1:main.CreationGenerateInput):Promise<creation.GenerateResult>",
  ],
  [
    "ConfirmCreationArchive declaration",
    apiDeclaration,
    "ConfirmCreationArchive(arg1:main.CreationArchiveInput):Promise<creation.ArchiveResult>",
  ],
  ["CreationWorkbench runtime binding", apiRuntime, "export function CreationWorkbench()"],
  ["RecoverCreation runtime binding", apiRuntime, "export function RecoverCreation()"],
  ["SelectCreationDirection card declaration", apiDeclaration, "SelectCreationDirection(arg1:string,arg2:string):Promise<creation.WorkbenchView>"],
  ["SelectCreationDirection card binding", apiRuntime, "export function SelectCreationDirection(arg1, arg2)"],
  ["KeepCreationDirection declaration", apiDeclaration, "KeepCreationDirection(arg1:string):Promise<creation.WorkbenchView>"],
  ["UnkeepCreationDirection declaration", apiDeclaration, "UnkeepCreationDirection(arg1:string):Promise<creation.WorkbenchView>"],
  [
    "ConfirmCreationGenerate runtime binding",
    apiRuntime,
    "export function ConfirmCreationGenerate(arg1)",
  ],
  ["CreationGenerateInput model", models, "export class CreationGenerateInput"],
  ["CreationArchiveInput model", models, "export class CreationArchiveInput"],
  ["Creation WorkbenchView model", models, "export class WorkbenchView"],
];

for (const [name, source, value] of expected) {
  if (!source.includes(value)) {
    throw new Error(`generated Wails ${name} is stale or malformed`);
  }
}

console.log("Verified generated Wails project lifecycle and creation bindings.");
