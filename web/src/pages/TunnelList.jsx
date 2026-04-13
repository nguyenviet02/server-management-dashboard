import { useEffect, useState } from 'react'
import { Box, Flex, Text, Button, Card, Heading, Table, Badge, Dialog, TextField } from '@radix-ui/themes'
import { Plus, Waypoints, Pencil, Trash2, FolderOpen } from 'lucide-react'
import { useNavigate } from 'react-router'
import { tunnelAPI } from '../api/index.js'

const emptyForm = {
    name: '',
    config_path: '',
    credential_path: '',
    shared_credential_key: '',
}

export default function TunnelList() {
    const navigate = useNavigate()
    const [tunnels, setTunnels] = useState([])
    const [loading, setLoading] = useState(true)
    const [dialogOpen, setDialogOpen] = useState(false)
    const [editing, setEditing] = useState(null)
    const [form, setForm] = useState(emptyForm)
    const [saving, setSaving] = useState(false)

    const load = async () => {
        try {
            const res = await tunnelAPI.listTunnels()
            setTunnels(res.data?.tunnels || [])
        } finally {
            setLoading(false)
        }
    }

    useEffect(() => {
        load()
    }, [])

    const openCreate = () => {
        setEditing(null)
        setForm(emptyForm)
        setDialogOpen(true)
    }

    const openEdit = (tunnel) => {
        setEditing(tunnel)
        setForm({
            name: tunnel.name || '',
            config_path: tunnel.config_path || '',
            credential_path: tunnel.credential_path || '',
            shared_credential_key: tunnel.shared_credential_key || '',
        })
        setDialogOpen(true)
    }

    const save = async () => {
        setSaving(true)
        try {
            if (editing) {
                await tunnelAPI.updateTunnel(editing.id, form)
            } else {
                await tunnelAPI.createTunnel(form)
            }
            setDialogOpen(false)
            load()
        } catch (e) {
            alert(e.response?.data?.error || e.message || 'Operation failed')
        } finally {
            setSaving(false)
        }
    }

    const remove = async (tunnel) => {
        if (!confirm(`Delete tunnel record for ${tunnel.name}? Files on disk will not be removed.`)) return
        try {
            await tunnelAPI.deleteTunnel(tunnel.id)
            load()
        } catch (e) {
            alert(e.response?.data?.error || e.message || 'Operation failed')
        }
    }

    return (
        <Box>
            <Flex justify="between" align="center" mb="4">
                <Box>
                    <Heading size="5">Cloudflare Tunnels</Heading>
                    <Text size="2" color="gray">Register and manage multiple tunnel config files.</Text>
                </Box>
                <Button onClick={openCreate}>
                    <Plus size={16} /> Register tunnel
                </Button>
            </Flex>

            {loading ? (
                <Text color="gray">Loading…</Text>
            ) : tunnels.length === 0 ? (
                <Card>
                    <Flex direction="column" align="center" gap="3" py="6">
                        <Waypoints size={48} strokeWidth={1} color="var(--gray-8)" />
                        <Text size="3" color="gray">No tunnels registered yet.</Text>
                        <Button onClick={openCreate}><Plus size={16} /> Register tunnel</Button>
                    </Flex>
                </Card>
            ) : (
                <Table.Root variant="surface">
                    <Table.Header>
                        <Table.Row>
                            <Table.ColumnHeaderCell>Name</Table.ColumnHeaderCell>
                            <Table.ColumnHeaderCell>Tunnel</Table.ColumnHeaderCell>
                            <Table.ColumnHeaderCell>Config</Table.ColumnHeaderCell>
                            <Table.ColumnHeaderCell>Credential</Table.ColumnHeaderCell>
                            <Table.ColumnHeaderCell>Actions</Table.ColumnHeaderCell>
                        </Table.Row>
                    </Table.Header>
                    <Table.Body>
                        {tunnels.map((tunnel) => (
                            <Table.Row key={tunnel.id} style={{ cursor: 'pointer' }} onClick={() => navigate(`/tunnels/${tunnel.id}`)}>
                                <Table.Cell><Text weight="medium">{tunnel.name}</Text></Table.Cell>
                                <Table.Cell><Badge variant="soft">{tunnel.tunnel_name}</Badge></Table.Cell>
                                <Table.Cell><Text size="2" style={{ wordBreak: 'break-all' }}>{tunnel.config_path}</Text></Table.Cell>
                                <Table.Cell><Text size="2" style={{ wordBreak: 'break-all' }}>{tunnel.credential_path}</Text></Table.Cell>
                                <Table.Cell onClick={(e) => e.stopPropagation()}>
                                    <Flex gap="2">
                                        <Button size="1" variant="soft" onClick={() => navigate(`/tunnels/${tunnel.id}`)}>
                                            <FolderOpen size={14} /> Open
                                        </Button>
                                        <Button size="1" variant="soft" onClick={() => openEdit(tunnel)}>
                                            <Pencil size={14} /> Edit
                                        </Button>
                                        <Button size="1" color="red" variant="soft" onClick={() => remove(tunnel)}>
                                            <Trash2 size={14} /> Delete
                                        </Button>
                                    </Flex>
                                </Table.Cell>
                            </Table.Row>
                        ))}
                    </Table.Body>
                </Table.Root>
            )}

            <Dialog.Root open={dialogOpen} onOpenChange={setDialogOpen}>
                <Dialog.Content maxWidth="520px">
                    <Dialog.Title>{editing ? 'Edit tunnel' : 'Register tunnel'}</Dialog.Title>
                    <Dialog.Description size="2" mb="4">
                        Register an existing cloudflared config file and origincert file.
                    </Dialog.Description>

                    <Flex direction="column" gap="3">
                        <label>
                            <Text as="div" size="2" mb="1" weight="medium">Display name</Text>
                            <TextField.Root value={form.name} onChange={(e) => setForm((current) => ({ ...current, name: e.target.value }))} />
                        </label>
                        <label>
                            <Text as="div" size="2" mb="1" weight="medium">Config path</Text>
                            <TextField.Root value={form.config_path} onChange={(e) => setForm((current) => ({ ...current, config_path: e.target.value }))} />
                        </label>
                        <label>
                            <Text as="div" size="2" mb="1" weight="medium">Credential path</Text>
                            <TextField.Root value={form.credential_path} onChange={(e) => setForm((current) => ({ ...current, credential_path: e.target.value }))} />
                        </label>
                        <label>
                            <Text as="div" size="2" mb="1" weight="medium">Shared credential key</Text>
                            <TextField.Root value={form.shared_credential_key} onChange={(e) => setForm((current) => ({ ...current, shared_credential_key: e.target.value }))} />
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
