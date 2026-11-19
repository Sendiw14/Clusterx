import { MarkdownDoc, docMetadata } from "@/features/docs";

export const metadata = docMetadata("concepts");

export default function Page() {
  return <MarkdownDoc slug="concepts" />;
}
