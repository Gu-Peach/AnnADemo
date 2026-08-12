import { NOTES_STORAGE_KEY } from '../constants';
import type { AnnaRuntime, Note } from '../types';

export class NotesRepository {
  constructor(private readonly anna: AnnaRuntime) {}

  async loadNotes(): Promise<Note[]> {
    const response = await this.anna.storage.get({ key: NOTES_STORAGE_KEY });
    if (response.value == null) {
      return [];
    }
    if (!Array.isArray(response.value)) {
      throw new Error('Stored notes are invalid.');
    }
    return response.value.map(normalizeNote).sort((left, right) => left.order - right.order);
  }

  async saveNotes(notes: Note[]): Promise<void> {
    await this.anna.storage.set({ key: NOTES_STORAGE_KEY, value: notes });
  }
}

export function createNote(content: string, existingNotes: Note[], now = new Date()): Note {
  const trimmed = content.trim();
  if (!trimmed) {
    throw new Error('Note content is required.');
  }
  const nextOrder = existingNotes.reduce((max, note) => Math.max(max, note.order), 0) + 1;
  return {
    id: `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 10)}`,
    order: nextOrder,
    content: trimmed,
    createdAt: now.toISOString(),
  };
}

function normalizeNote(value: unknown): Note {
  if (!value || typeof value !== 'object') {
    throw new Error('Stored note is invalid.');
  }
  const candidate = value as Partial<Note>;
  if (typeof candidate.content !== 'string') {
    throw new Error('Stored note content is invalid.');
  }
  return {
    id: typeof candidate.id === 'string' ? candidate.id : crypto.randomUUID(),
    order: typeof candidate.order === 'number' ? candidate.order : 0,
    content: candidate.content,
    createdAt: typeof candidate.createdAt === 'string' ? candidate.createdAt : new Date().toISOString(),
  };
}
