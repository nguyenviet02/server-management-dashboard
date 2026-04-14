import { useCallback, useEffect, useMemo, useState } from 'react'
import {
    Box, Button, Callout, Card, Flex, Heading, IconButton, Switch, Table, Text, TextField,
} from '@radix-ui/themes'
import { Clock, Plus, RotateCcw, Save, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { cronjobAPI } from '../api/index.js'

const newEntry = () => ({
    id: `new-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
    line_number: 0,
    schedule: '',
    command: '',
    enabled: true,
})

export default function CronJobManager() {
    const { t } = useTranslation()
    const [entries, setEntries] = useState([])
    const [sourceHash, setSourceHash] = useState('')
    const [hasUnmanagedLines, setHasUnmanagedLines] = useState(false)
    const [unmanagedLineCount, setUnmanagedLineCount] = useState(0)
    const [loading, setLoading] = useState(true)
    const [saving, setSaving] = useState(false)
    const [error, setError] = useState('')

    const fetchCrontab = useCallback(async () => {
        setLoading(true)
        setError('')
        try {
            const res = await cronjobAPI.getCrontab()
            setEntries(res.data?.entries || [])
            setSourceHash(res.data?.source_hash || '')
            setHasUnmanagedLines(!!res.data?.has_unmanaged_lines)
            setUnmanagedLineCount(res.data?.unmanaged_line_count || 0)
        } catch {
            setEntries([])
            setSourceHash('')
            setHasUnmanagedLines(false)
            setUnmanagedLineCount(0)
            setError('')
        } finally {
            setLoading(false)
        }
    }, [])

    useEffect(() => {
        fetchCrontab()
    }, [fetchCrontab])

    const hasNoEntries = entries.length === 0

    const hasInvalidEntries = useMemo(
        () => hasNoEntries || entries.some(entry => !entry.schedule.trim() || !entry.command.trim()),
        [entries, hasNoEntries]
    )

    const updateEntry = (id, field, value) => {
        setEntries(current => current.map(entry => (entry.id === id ? { ...entry, [field]: value } : entry)))
    }

    const addEntry = () => {
        setEntries(current => [...current, newEntry()])
    }

    const removeEntry = (id) => {
        setEntries(current => current.filter(entry => entry.id !== id))
    }

    const handleSave = async () => {
        if (hasNoEntries) {
            setError(t('cronjob.empty_error'))
            return
        }

        setSaving(true)
        setError('')
        try {
            const payload = {
                source_hash: sourceHash,
                entries: entries.map(entry => ({
                    id: entry.id,
                    line_number: entry.line_number || 0,
                    schedule: entry.schedule.trim(),
                    command: entry.command.trim(),
                    enabled: entry.enabled !== false,
                })),
            }
            const res = await cronjobAPI.saveCrontab(payload)
            setEntries(res.data?.entries || [])
            setSourceHash(res.data?.source_hash || '')
            setHasUnmanagedLines(!!res.data?.has_unmanaged_lines)
            setUnmanagedLineCount(res.data?.unmanaged_line_count || 0)
        } catch (e) {
            setError(e?.response?.data?.error || e.message)
        } finally {
            setSaving(false)
        }
    }

    return (
        <Box p="4" style={{ maxWidth: 1200, margin: '0 auto' }}>
            <Flex justify="between" align="center" mb="4">
                <Box>
                    <Heading size="5"><Clock size={20} style={{ display: 'inline', marginRight: 8, verticalAlign: 'text-bottom' }} />{t('cronjob.title')}</Heading>
                    <Text size="2" color="gray">{t('cronjob.subtitle')}</Text>
                </Box>
                <Flex gap="2">
                    <Button variant="soft" onClick={fetchCrontab} disabled={loading || saving}>
                        <RotateCcw size={14} /> {t('common.refresh')}
                    </Button>
                    <Button variant="soft" onClick={addEntry} disabled={saving}>
                        <Plus size={14} /> {t('common.add')}
                    </Button>
                    <Button onClick={handleSave} disabled={loading || saving || hasInvalidEntries}>
                        <Save size={14} /> {saving ? t('common.saving') : t('common.save')}
                    </Button>
                </Flex>
            </Flex>

            {error && (
                <Callout.Root color="red" mb="3">
                    <Callout.Text>{error}</Callout.Text>
                </Callout.Root>
            )}

            {hasUnmanagedLines && (
                <Callout.Root color="amber" mb="3">
                    <Callout.Text>
                        {t('cronjob.unmanaged_warning', { count: unmanagedLineCount })}
                    </Callout.Text>
                </Callout.Root>
            )}

            <Card>
                {loading ? (
                    <Text size="2" color="gray">{t('common.loading')}</Text>
                ) : entries.length === 0 ? (
                    <Flex direction="column" gap="3" align="start">
                        <Callout.Root color="gray">
                            <Callout.Text>{t('cronjob.no_tasks')}</Callout.Text>
                        </Callout.Root>
                        <Button onClick={addEntry}>
                            <Plus size={14} /> {t('cronjob.add_first_entry')}
                        </Button>
                    </Flex>
                ) : (
                    <Table.Root>
                        <Table.Header>
                            <Table.Row>
                                <Table.ColumnHeaderCell>{t('cronjob.expression')}</Table.ColumnHeaderCell>
                                <Table.ColumnHeaderCell>{t('cronjob.command')}</Table.ColumnHeaderCell>
                                <Table.ColumnHeaderCell>{t('cronjob.enabled')}</Table.ColumnHeaderCell>
                                <Table.ColumnHeaderCell>{t('common.actions')}</Table.ColumnHeaderCell>
                            </Table.Row>
                        </Table.Header>
                        <Table.Body>
                            {entries.map(entry => {
                                const isInvalid = !entry.schedule.trim() || !entry.command.trim()
                                return (
                                    <Table.Row key={entry.id}>
                                        <Table.Cell style={{ width: '28%' }}>
                                            <TextField.Root
                                                value={entry.schedule}
                                                onChange={(e) => updateEntry(entry.id, 'schedule', e.target.value)}
                                                placeholder="*/5 * * * *"
                                                color={isInvalid && !entry.schedule.trim() ? 'red' : undefined}
                                                style={{ fontFamily: 'monospace' }}
                                            />
                                        </Table.Cell>
                                        <Table.Cell>
                                            <TextField.Root
                                                value={entry.command}
                                                onChange={(e) => updateEntry(entry.id, 'command', e.target.value)}
                                                placeholder={t('cronjob.command_placeholder')}
                                                color={isInvalid && !entry.command.trim() ? 'red' : undefined}
                                                style={{ fontFamily: 'monospace' }}
                                            />
                                        </Table.Cell>
                                        <Table.Cell style={{ width: 120 }}>
                                            <Switch
                                                checked={entry.enabled !== false}
                                                onCheckedChange={(checked) => updateEntry(entry.id, 'enabled', checked)}
                                            />
                                        </Table.Cell>
                                        <Table.Cell style={{ width: 80 }}>
                                            <IconButton variant="ghost" color="red" onClick={() => removeEntry(entry.id)}>
                                                <Trash2 size={14} />
                                            </IconButton>
                                        </Table.Cell>
                                    </Table.Row>
                                )
                            })}
                        </Table.Body>
                    </Table.Root>
                )}
            </Card>

            <Text size="1" color="gray" mt="2" style={{ display: 'block' }}>
                {t('cronjob.expression_help')}
            </Text>
            <Text size="1" color="gray" style={{ display: 'block' }}>
                {t('cronjob.expression_examples')}
            </Text>
        </Box>
    )
}
