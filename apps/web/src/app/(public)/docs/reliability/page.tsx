import { MarkdownDoc, docMetadata } from "@/features/docs";

export const metadata = docMetadata("reliability");

export default function Page() {
  return <MarkdownDoc slug="reliability" />;
}
