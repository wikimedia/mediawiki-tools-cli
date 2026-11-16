import { llmsIndex } from '../../../scripts/markdown-docs.mjs';
import { markdownDocuments } from '../../markdown-documents';

export function getStaticPaths() {
  const documentation = markdownDocuments();
  return [undefined, 'guides', 'manual'].map((scope) => ({
    params: { scope },
    props: { text: llmsIndex(documentation, scope ? `/${scope}/` : '/') },
  }));
}

export function GET({ props }: { props: { text: string } }) {
  return new Response(props.text, { headers: { 'Content-Type': 'text/plain; charset=utf-8' } });
}
