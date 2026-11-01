import { defineCollection } from 'astro:content';
import { z } from 'astro/zod';
import { docsLoader } from '@astrojs/starlight/loaders';
import { docsSchema } from '@astrojs/starlight/schema';

export const collections = {
  docs: defineCollection({
    loader: docsLoader({
      generateId: ({ entry }) => entry.replace(/\.(md|mdx)$/, ''),
    }),
    schema: docsSchema({
      extend: z.object({
        guideSource: z.string().optional(),
        guideCommand: z.string().optional(),
        manualVersion: z.string().optional(),
        manualUnreleased: z.boolean().optional(),
      }),
    }),
  }),
};
