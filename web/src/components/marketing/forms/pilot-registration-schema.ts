import { z } from 'zod';

export const ROLE_OPTIONS = [
  'investigator',
  'prosecutor',
  'defense',
  'analyst',
  'court_admin',
  'other',
] as const;

export type PilotRole = (typeof ROLE_OPTIONS)[number];

export const pilotRegistrationSchema = z.object({
  name: z.string().min(2, 'Name must be at least 2 characters').max(100),
  email: z.string().email('Please enter a valid email address'),
  organization: z
    .string()
    .min(2, 'Organization must be at least 2 characters')
    .max(160),
  role: z.enum(ROLE_OPTIONS),
  message: z
    .string()
    .min(20, 'Please provide at least 20 characters')
    .max(2000),
  locale: z.enum(['en', 'fr']),
  honeypot: z.string().max(0).optional(),
});

export type PilotRegistrationFormData = z.infer<
  typeof pilotRegistrationSchema
>;
