import { writeAxeEvidence } from "./axe-report";
import { stopStack } from "./stack/stack";

export default async function globalTeardown() {
  writeAxeEvidence();
  await stopStack();
}
