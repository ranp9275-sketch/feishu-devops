import React, { useEffect, useState } from 'react';
import { Table, Button, Space, Typography, message, Modal, Form, Input } from 'antd';
import { PlusOutlined, EditOutlined, DeleteOutlined, ReloadOutlined, KeyOutlined } from '@ant-design/icons';
import { getRobots, addRobot, updateRobot, deleteRobot, updateFeishuToken } from '../services/api';

const { Title } = Typography;

const FeishuConfig: React.FC = () => {
  const [data, setData] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [editingRecord, setEditingRecord] = useState<any>(null);
  const [form] = Form.useForm();

  // Token Config
  const [tokenForm] = Form.useForm();
  const [tokenModalOpen, setTokenModalOpen] = useState(false);

  const fetchData = async () => {
    setLoading(true);
    try {
      const res = await getRobots({ page: 1, page_size: 100 });
      if (res.code === 0 && res.data && Array.isArray(res.data.items)) {
         setData(res.data.items);
      } else {
         setData([]);
      }
    } catch (error) {
      message.error('Failed to load robots');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchData();
  }, []);

  const handleAdd = () => {
    setEditingRecord(null);
    form.resetFields();
    setModalOpen(true);
  };

  const handleEdit = (record: any) => {
    setEditingRecord(record);
    form.setFieldsValue(record);
    setModalOpen(true);
  };

  const handleDelete = async (name: string) => {
    try {
      await deleteRobot({ name });
      message.success('Robot deleted successfully');
      fetchData();
    } catch (error) {
      message.error('Failed to delete robot');
    }
  };

  const handleSave = async () => {
    try {
      const values = await form.validateFields();
      if (editingRecord) {
        await updateRobot({ ...values, name: editingRecord.name });
        message.success('Robot updated successfully');
      } else {
        await addRobot(values);
        message.success('Robot created successfully');
      }
      setModalOpen(false);
      fetchData();
    } catch (error) {
      console.error(error);
      message.error('Failed to save robot');
    }
  };

  const handleTokenSave = async () => {
    try {
      const values = await tokenForm.validateFields();
      await updateFeishuToken(values);
      message.success('Global Token Updated Successfully');
      setTokenModalOpen(false);
      tokenForm.resetFields();
    } catch (error) {
      message.error('Failed to update token');
    }
  };

  const columns = [
    { title: 'Name', dataIndex: 'name', key: 'name' },
    { title: 'App ID', dataIndex: 'app_id', key: 'app_id' },
    { title: 'Project', dataIndex: 'project', key: 'project' },
    { title: 'Webhook', dataIndex: 'webhook', key: 'webhook', ellipsis: true },
    { 
      title: 'Actions', 
      key: 'actions',
      render: (_: any, record: any) => (
        <Space>
          <Button icon={<EditOutlined />} onClick={() => handleEdit(record)} />
          <Button icon={<DeleteOutlined />} danger onClick={() => handleDelete(record.name)} />
        </Space>
      )
    }
  ];

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
        <Title level={2}>Feishu Configurations</Title>
        <Space>
           <Button icon={<KeyOutlined />} onClick={() => setTokenModalOpen(true)}>Update User Token</Button>
           <Button icon={<ReloadOutlined />} onClick={fetchData}>Refresh</Button>
           <Button type="primary" icon={<PlusOutlined />} onClick={handleAdd}>Add Robot</Button>
        </Space>
      </div>

      <Table columns={columns} dataSource={data} rowKey="name" loading={loading} />

      {/* Robot Modal */}
      <Modal
        title={editingRecord ? 'Edit Robot' : 'Add Robot'}
        open={modalOpen}
        onOk={handleSave}
        onCancel={() => setModalOpen(false)}
      >
        <Form form={form} layout="vertical">
          <Form.Item name="name" label="Name" rules={[{ required: true }]}>
            <Input disabled={!!editingRecord} />
          </Form.Item>
          <Form.Item name="app_id" label="App ID">
            <Input />
          </Form.Item>
          <Form.Item name="app_secret" label="App Secret">
            <Input.Password />
          </Form.Item>
          <Form.Item name="project" label="Project">
            <Input />
          </Form.Item>
          <Form.Item name="webhook" label="Webhook URL" rules={[{ required: true }]}>
            <Input.TextArea rows={3} />
          </Form.Item>
        </Form>
      </Modal>

      {/* Token Modal */}
      <Modal
        title="Update Global Feishu User Token"
        open={tokenModalOpen}
        onOk={handleTokenSave}
        onCancel={() => setTokenModalOpen(false)}
        footer={[
          <Button key="cancel" onClick={() => setTokenModalOpen(false)}>Cancel</Button>,
          <Button key="submit" type="primary" onClick={handleTokenSave}>Update</Button>
        ]}
      >
        <div style={{ marginBottom: 16, color: '#faad14' }}>
          Note: This token is used for creating group chats and searching users. 
          The Tenant Token is managed automatically.
          <br/>
          <strong>Recommendation:</strong> Provide a <strong>Refresh Token</strong> for long-term validity.
        </div>
        <Form form={tokenForm} layout="vertical">
          <Form.Item name="user_access_token" label="User Access Token (Optional)">
            <Input.TextArea rows={2} placeholder="Temporary access token..." />
          </Form.Item>
          <Form.Item name="user_refresh_token" label="User Refresh Token (Recommended)">
            <Input.TextArea rows={2} placeholder="Long-term refresh token..." />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default FeishuConfig;
