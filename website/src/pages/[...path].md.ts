import { markdownPath } from '../../scripts/markdown-docs.mjs';
import { markdownDocuments } from '../markdown-documents';

export function getStaticPaths() {
  return markdownDocuments().documents.map((page) => ({
    params: { path: markdownPath(page.pathname).slice(1, -3) },
    props: { markdown: page.markdown },
  }));
}

export function GET({ props }: { props: { markdown: string } }) {
  return new Response(props.markdown, { headers: { 'Content-Type': 'text/markdown; charset=utf-8' } });
}
