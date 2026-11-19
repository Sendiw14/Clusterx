import { resetAxeLog } from "./axe-report";
import { startStack, stopStack } from "./stack/stack";

export default async function globalSetup() {
  resetAxeLog();
  try {
    await startStack();
  } catch (e) {
    // Playwright does not run global teardown when setup throws: never leave a database or Go process behind.
    await stopStack();
    throw e;
  }
}
