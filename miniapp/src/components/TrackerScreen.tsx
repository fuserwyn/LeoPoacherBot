...
  useEffect(() => {
    const onPaste = (e: ClipboardEvent) => {
      const item = Array.from(e.clipboardData?.items ?? []).find((i) =>
        i.type.startsWith('image/'),
      );
      if (!item) return;
      const file = item.getAsFile();
      if (!file) return;
      const target: 'new' | number | null = detail ? detail.id : tab === 'task' ? 'new' : null;
      if (target === null) return;
      e.preventDefault();
      setPasted(file);
      setEditorFor(target);
    };
    window.addEventListener('paste', onPaste);
    return () => window.removeEventListener('paste', onPaste);
  }, [detail, tab]);
...