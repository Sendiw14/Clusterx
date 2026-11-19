import { MarkdownDoc, docMetadata } from "@/features/docs";

export const metadata = docMetadata("security");

export default function Page() {
  return <MarkdownDoc slug="security" />;
}
