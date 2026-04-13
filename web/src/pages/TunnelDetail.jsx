import { useEffect, useMemo, useState } from 'react'
import { Box, Flex, Text, Button, Card, Heading, Table, Badge, Dialog, TextField, Callout } from '@radix-ui/themes'
import { ArrowLeft, Plus, Trash2, Pencil, RotateCw, AlertTriangle } from 'lucide-react'
import { useNavigate, useParams } from 'react-router'
import { tunnelAPI } from '../api/index.js'

const emptyIngress = {
    id: '',
    hostname: '',
    service: '',
}

export default function TunnelDetail() {
    const navigate = useNavigate()
    const { id } = useParams()
    const [tunnel, setTunnel] = useState(null)
    const [config, setConfig] = useState(null)
    const [status, setStatus] = useState(null)
    const [loading, setLoading] = useState(true)
    const [dialogOpen, setDialogOpen] = useState(false)
    const [editing, setEditing] = useState(null)
    const [form, setForm] = useState(emptyIngress)
    const [saving, setSaving] = useState(false)
    const hasServiceName = Boolean(tunnel?.service_name?.trim())

    const load = async () => {
        try {
            const detailRes = await tunnelAPI.getTunnelConfig(id)
            const nextTunnel = detailRes.data?.tunnel || null
            setTunnel(nextTunnel)
            setConfig(detailRes.data?.config || null)
            if (nextTunnel?.service_name?.trim()) {
                const statusRes = await tunnelAPI.serviceStatus(id)
                setStatus(statusRes.data || null)
            } else {
                setStatus(null)
            }
        } catch (e) {
            alert(e.response?.data?.error || e.message || 'Failed to load tunnel')
            navigate('/tunnels')
        } finally {
            setLoading(false)
        }
    }

    useEffect(() => {
        load()
    }, [id])

    const visibleIngress = useMemo(() => {
        const ingress = config?.ingress || []
        return ingress.filter((entry) => !(entry.service === 'http_status:404' && !entry.hostname))
    }, [config])

    const openCreate = () => {
        setEditing(null)
        setForm(emptyIngress)
        setDialogOpen(true)
    }

    const openEdit = (entry) => {
        setEditing(entry)
        setForm(entry)
        setDialogOpen(true)
    }

    const save = async () => {
        setSaving(true)
        try {
            const payload = { item: { ...form, id: editing?.id || undefined }, route_dns: true }
            const res = editing
                ? await tunnelAPI.updateIngress(id, payload)
                : await tunnelAPI.addIngress(id, payload)
            setTunnel(res.data?.tunnel || null)
            setConfig(res.data?.config || null)
            setDialogOpen(false)
            if (res.data?.warning) alert(res.data.warning)
        } catch (e) {
            alert(e.response?.data?.error || e.message || 'Operation failed')
        } finally {
            setSaving(false)
        }
    }

    const remove = async (entry) => {
        if (!confirm(`Delete ingress for ${entry.hostname || entry.service}?`)) return
        try {
            const res = await tunnelAPI.deleteIngress(id, entry.id)
            setTunnel(res.data?.tunnel || null)
            setConfig(res.data?.config || null)
            if (res.data?.warning) alert(res.data.warning)
        } catch (e) {
            alert(e.response?.data?.error || e.message || 'Operation failed')
        }
    }

    const restart = async () => {
        try {
            await tunnelAPI.restartService(id)
            load()
        } catch (e) {
            alert(e.response?.data?.error || e.message || 'Restart failed')
        }
    }

    if (loading) {
        return <Text color="gray">Loading…</Text>
    }

    return (
        <Box>
            <Flex justify="between" align="center" mb="4">
                <Flex align="center" gap="3">
                    <Button variant="soft" onClick={() => navigate('/tunnels')}>
                        <ArrowLeft size={16} /> Back
                    </Button>
                    <Box>
                        <Heading size="5">{tunnel?.name || 'Tunnel detail'}</Heading>
                        <Text size="2" color="gray">Manage ingress routes for this tunnel.</Text>
                    </Box>
                </Flex>
                {hasServiceName ? (
                    <Button onClick={restart}>
                        <RotateCw size={16} /> Restart {tunnel.service_name}
                    </Button>
                ) : null}
            </Flex>

            <Flex direction="column" gap="4">
                <Card>
                    <Flex direction="column" gap="3">
                        <Heading size="3">Tunnel overview</Heading>
                        <Flex gap="3" wrap="wrap">
                            <Badge variant="soft">{config?.tunnel_name || tunnel?.tunnel_name}</Badge>
                            {hasServiceName ? (
                                <>
                                    <Badge color={status?.active === 'active' ? 'green' : 'gray'}>{status?.active || 'unknown'}</Badge>
                                    <Badge color={status?.enabled === 'enabled' ? 'green' : 'gray'}>{status?.enabled || 'unknown'}</Badge>
                                </>
                            ) : null}
                        </Flex>
                        <Text size="2"><strong>Config path:</strong> {tunnel?.config_path}</Text>
                        <Text size="2"><strong>Credential path:</strong> {tunnel?.credential_path}</Text>
                        {tunnel?.shared_credential_key ? (
                            <Text size="2"><strong>Shared credential key:</strong> {tunnel.shared_credential_key}</Text>
                        ) : null}
                        {hasServiceName ? (
                            <Text size="2"><strong>Service name:</strong> {tunnel.service_name}</Text>
                        ) : (
                            <Callout.Root color="orange">
                                <Callout.Icon><AlertTriangle size={16} /></Callout.Icon>
                                <Callout.Text>Add a service name in tunnel settings to enable status and restart.</Callout.Text>
                            </Callout.Root>
                        )}
                    </Flex>
                </Card>

                <Card>
                    <Flex justify="between" align="center" mb="3">
                        <Box>
                            <Heading size="3">Ingress routes</Heading>
                            <Text size="2" color="gray">Hostname and service rules from the config file.</Text>
                        </Box>
                        <Button onClick={openCreate}><Plus size={16} /> Add route</Button>
                    </Flex>

                    {visibleIngress.length === 0 ? (
                        <Text color="gray">No ingress entries found yet.</Text>
                    ) : (
                        <Table.Root variant="surface">
                            <Table.Header>
                                <Table.Row>
                                    <Table.ColumnHeaderCell>Hostname</Table.ColumnHeaderCell>
                                    <Table.ColumnHeaderCell>Service</Table.ColumnHeaderCell>
                                    <Table.ColumnHeaderCell>Actions</Table.ColumnHeaderCell>
                                </Table.Row>
                            </Table.Header>
                            <Table.Body>
                                {visibleIngress.map((entry) => (
                                    <Table.Row key={entry.id}>
                                        <Table.Cell>{entry.hostname || '—'}</Table.Cell>
                                        <Table.Cell>{entry.service}</Table.Cell>
                                        <Table.Cell>
                                            <Flex gap="2">
                                                <Button size="1" variant="soft" onClick={() => openEdit(entry)}>
                                                    <Pencil size={14} /> Edit
                                                </Button>
                                                <Button size="1" color="red" variant="soft" onClick={() => remove(entry)}>
                                                    <Trash2 size={14} /> Delete
                                                </Button>
                                            </Flex>
                                        </Table.Cell>
                                    </Table.Row>
                                ))}
                            </Table.Body>
                        </Table.Root>
                    )}
                </Card>
            </Flex>

            <Dialog.Root open={dialogOpen} onOpenChange={setDialogOpen}>
                <Dialog.Content maxWidth="520px">
                    <Dialog.Title>{editing ? 'Edit ingress route' : 'Add DNS route'}</Dialog.Title>
                    <Dialog.Description size="2" mb="4">
                        Save the ingress rule into the tunnel config and create its DNS route.
                    </Dialog.Description>

                    <Flex direction="column" gap="3">
                        <label>
                            <Text as="div" size="2" mb="1" weight="medium">Hostname</Text>
                            <TextField.Root value={form.hostname} onChange={(e) => setForm((current) => ({ ...current, hostname: e.target.value }))} />
                        </label>
                        <label>
                            <Text as="div" size="2" mb="1" weight="medium">Service</Text>
                            <TextField.Root value={form.service} onChange={(e) => setForm((current) => ({ ...current, service: e.target.value }))} />
                        </label>
                    </Flex>

                    <Flex gap="3" mt="4" justify="end">
                        <Dialog.Close>
                            <Button variant="soft" color="gray">Cancel</Button>
                        </Dialog.Close>
                        <Button onClick={save} disabled={saving}>{saving ? 'Saving…' : 'Save'}</Button>
                    </Flex>
                </Dialog.Content>
            </Dialog.Root>
        </Box>
    )
}
