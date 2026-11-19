import { MarkdownDoc, docMetadata } from "@/features/docs";

export const metadata = docMetadata("workers");

export default function Page() {
  return <MarkdownDoc slug="workers" />;
}
