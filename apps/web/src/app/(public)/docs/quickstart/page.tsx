import { MarkdownDoc, docMetadata } from "@/features/docs";

export const metadata = docMetadata("quickstart");

export default function Page() {
  return <MarkdownDoc slug="quickstart" />;
}
