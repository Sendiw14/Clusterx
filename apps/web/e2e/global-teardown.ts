import { writeAxeEvidence } from "./axe-report";
export default function globalTeardown() { writeAxeEvidence(); }
