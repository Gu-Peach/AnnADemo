import { FormEvent, useEffect, useMemo, useState } from 'react';
import { createNote, NotesRepository } from './services/notesRepository';
import { SummarizeService } from './services/summarizeService';
import type { AnnaRuntime, Note, SummarizeResult } from './types';
import './styles.css';

interface AppProps {
  anna: AnnaRuntime;
}

export function App({ anna }: AppProps) {
  const notesRepository = useMemo(() => new NotesRepository(anna), [anna]);
  const summarizeService = useMemo(() => new SummarizeService(anna), [anna]);
  const [notes, setNotes] = useState<Note[]>([]);
  const [draft, setDraft] = useState('');
  const [summary, setSummary] = useState<SummarizeResult | null>(null);
  const [status, setStatus] = useState('Loading notes…');
  const [error, setError] = useState('');
  const [isSaving, setIsSaving] = useState(false);
  const [isSummarizing, setIsSummarizing] = useState(false);

  useEffect(() => {
    let mounted = true;
    anna.window?.set_title?.({ title: 'Mini Notes' }).catch(() => undefined);
    notesRepository
      .loadNotes()
      .then((loadedNotes) => {
        if (!mounted) return;
        setNotes(loadedNotes);
        setStatus(loadedNotes.length ? `${loadedNotes.length} notes loaded.` : 'No notes yet.');
      })
      .catch((caught) => {
        if (!mounted) return;
        setError(messageFrom(caught));
        setStatus('Could not load notes.');
      });
    return () => {
      mounted = false;
    };
  }, [anna, notesRepository]);

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError('');
    if (!draft.trim()) {
      setError('Please enter a note before saving.');
      return;
    }
    setIsSaving(true);
    try {
      const latestNotes = await notesRepository.loadNotes();
      const nextNotes = [...latestNotes, createNote(draft, latestNotes)];
      await notesRepository.saveNotes(nextNotes);
      setNotes(nextNotes);
      setDraft('');
      setSummary(null);
      setStatus('Note saved through anna.storage.set.');
    } catch (caught) {
      setError(messageFrom(caught));
    } finally {
      setIsSaving(false);
    }
  }

  async function handleDelete(noteId: string) {
    setError('');
    try {
      const latestNotes = await notesRepository.loadNotes();
      const nextNotes = latestNotes.filter((note) => note.id !== noteId);
      await notesRepository.saveNotes(nextNotes);
      setNotes(nextNotes);
      setSummary(null);
      setStatus('Note deleted and storage updated.');
    } catch (caught) {
      setError(messageFrom(caught));
    }
  }

  async function handleSummarize() {
    setError('');
    setSummary(null);
    if (notes.length === 0) {
      setError('Add at least one note before summarizing.');
      return;
    }
    setIsSummarizing(true);
    setStatus('Calling anna.tools.invoke → Executa → sampling/createMessage…');
    try {
      const nextSummary = await summarizeService.summarize(notes);
      setSummary(nextSummary);
      setStatus('Summary returned from Executa sampling.');
    } catch (caught) {
      setError(messageFrom(caught));
      setStatus('Summarize call finished with an expected harness/tool error.');
    } finally {
      setIsSummarizing(false);
    }
  }

  return (
    <main className="shell">
      <section className="hero">
        <p className="eyebrow">Anna App Runtime Demo</p>
        <h1>Mini Notes</h1>
        <p className="subtitle">Notes persist through Anna storage. Summaries run through a bundled Executa Tool and host sampling.</p>
      </section>

      <form className="composer" onSubmit={handleSubmit} aria-label="Create note">
        <label htmlFor="note-input">New note</label>
        <textarea
          id="note-input"
          value={draft}
          onChange={(event) => setDraft(event.target.value)}
          placeholder="Example: Tomorrow follow up with customer"
          rows={3}
        />
        <button type="submit" disabled={isSaving}>{isSaving ? 'Saving…' : 'Save note'}</button>
      </form>

      <section className="panel" aria-live="polite">
        <div className="panelHeader">
          <div>
            <h2>Notes</h2>
            <p>{status}</p>
          </div>
          <button type="button" className="secondary" onClick={handleSummarize} disabled={isSummarizing || notes.length === 0}>
            {isSummarizing ? 'Summarizing…' : 'Summarize'}
          </button>
        </div>

        {error ? <div className="error" role="alert">{error}</div> : null}

        {notes.length === 0 ? (
          <div className="empty">No notes yet. Save the first one to test anna.storage.set.</div>
        ) : (
          <ol className="notes">
            {notes.map((note) => (
              <li key={note.id} className="noteCard">
                <span className="order">#{note.order}</span>
                <p>{note.content}</p>
                <time dateTime={note.createdAt}>{new Date(note.createdAt).toLocaleString()}</time>
                <button type="button" className="danger" onClick={() => handleDelete(note.id)} aria-label={`Delete note ${note.order}`}>
                  Delete
                </button>
              </li>
            ))}
          </ol>
        )}
      </section>

      {summary ? (
        <section className="summary">
          <h2>LLM Summary</h2>
          <p>{summary.summary}</p>
          {summary.model ? <small>Model: {summary.model}</small> : null}
        </section>
      ) : null}
    </main>
  );
}

function messageFrom(value: unknown): string {
  if (value instanceof Error) return value.message;
  if (typeof value === 'string') return value;
  try {
    return JSON.stringify(value);
  } catch {
    return 'Unknown error';
  }
}
