import { MarkdownDoc, docMetadata } from "@/features/docs";

export const metadata = docMetadata("deployment");

export default function Page() {
  return <MarkdownDoc slug="deployment" />;
}
