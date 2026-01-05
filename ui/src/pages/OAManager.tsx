import React, { useEffect, useState } from 'react';
import { Table, Button, Drawer, Space, Typography, message, Modal, Input, Tag } from 'antd';
import { EyeOutlined, UploadOutlined, ReloadOutlined } from '@ant-design/icons';
import { getJsonAll, storeJson } from '../services/api';

const { Title } = Typography;
const { TextArea } = Input;

const OAManager: React.FC = () => {
  const [data, setData] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [selectedRecord, setSelectedRecord] = useState<any>(null);
  const [modalOpen, setModalOpen] = useState(false);
  const [jsonInput, setJsonInput] = useState('');

  const fetchData = async () => {
    setLoading(true);
    try {
      const res = await getJsonAll();
      if (res.code === 200 && Array.isArray(res.data)) {
        // Sort by received_at desc
        const sorted = res.data.sort((a, b) => {
            const timeA = new Date(a.received_at).getTime();
            const timeB = new Date(b.received_at).getTime();
            // Handle invalid dates (Unknown or others)
            const validA = !isNaN(timeA);
            const validB = !isNaN(timeB);
            
            if (!validA && !validB) return 0;
            if (!validA) return 1; // Put invalid dates at the bottom
            if (!validB) return -1;
            
            return timeB - timeA;
        });
        setData(sorted);
      } else {
        setData([]);
      }
    } catch (error) {
      message.error('Failed to load OA data');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchData();
  }, []);

  const handleView = (record: any) => {
    setSelectedRecord(record);
    setDrawerOpen(true);
  };

  const handleStore = async () => {
    try {
      const parsed = JSON.parse(jsonInput);
      await storeJson(parsed);
      message.success('JSON stored successfully');
      setModalOpen(false);
      setJsonInput('');
      fetchData();
    } catch (e) {
      message.error('Invalid JSON format');
    }
  };

  const columns = [
    {
      title: 'ID',
      dataIndex: 'id',
      key: 'id',
      width: 150,
    },
    {
      title: 'Request Name',
      key: 'request_name',
      width: 250,
      render: (_: any, record: any) => record.parsed_request_name || record.original_data?.requestManager?.requestname || '-',
    },
    {
      title: 'Job Name',
      key: 'job_name',
      width: 200,
      render: (_: any, record: any) => record.parsed_job_name || '-',
    },
    {
      title: 'Applicant',
      key: 'applicant',
      width: 100,
      render: (_: any, record: any) => record.parsed_applicant || record.original_data?.requestManager?.sqr || '-',
    },
    {
      title: 'Request Time',
      key: 'request_time',
      width: 150,
      render: (_: any, record: any) => record.parsed_request_time || record.original_data?.requestManager?.sqsj || '-',
    },
    {
      title: 'Received At',
      dataIndex: 'received_at',
      key: 'received_at',
      width: 200,
      render: (text: string) => text === 'Unknown' ? <span style={{ color: 'red' }}>Unknown</span> : text,
    },
    {
      title: 'Processed',
      dataIndex: 'processed',
      key: 'processed',
      width: 100,
      render: (processed: boolean) => (
        <Tag color={processed ? 'green' : 'orange'}>
          {processed ? 'Yes' : 'No'}
        </Tag>
      ),
    },
    {
      title: 'IP Address',
      dataIndex: 'ip_address',
      key: 'ip_address',
      width: 150,
    },
    {
      title: 'Status',
      key: 'status',
      width: 100,
      render: (_: any, record: any) => (
        record.load_failed ? <span style={{ color: 'red' }}>Error</span> : <span style={{ color: 'green' }}>Valid</span>
      ),
    },
    {
      title: 'Action',
      key: 'action',
      render: (_: any, record: any) => (
        <Space size="middle">
          <Button icon={<EyeOutlined />} onClick={() => handleView(record)}>
            View
          </Button>
        </Space>
      ),
    },
  ];

  return (
    <div style={{ padding: '24px' }}>
      <Space style={{ marginBottom: 16, justifyContent: 'space-between', width: '100%' }}>
        <Title level={2} style={{ margin: 0 }}>OA Data Management</Title>
        <Space>
          <Button icon={<ReloadOutlined />} onClick={fetchData}>Refresh</Button>
          <Button type="primary" icon={<UploadOutlined />} onClick={() => setModalOpen(true)}>
            Store New JSON
          </Button>
        </Space>
      </Space>

      <Table 
        columns={columns} 
        dataSource={data} 
        rowKey="id" 
        loading={loading}
        pagination={{ pageSize: 10 }}
      />

      <Drawer
        title={selectedRecord?.load_failed ? "File Content (Parse Error)" : "JSON Details"}
        placement="right"
        width={600}
        onClose={() => setDrawerOpen(false)}
        open={drawerOpen}
      >
        {selectedRecord?.load_failed ? (
          <div>
            <Typography.Text type="danger" strong>
              Error: {selectedRecord.error}
            </Typography.Text>
            <div style={{ marginTop: 16 }}>
              <Typography.Text strong>Raw Content:</Typography.Text>
              <pre style={{ 
                marginTop: 8, 
                backgroundColor: '#f5f5f5', 
                padding: 12, 
                borderRadius: 4,
                overflow: 'auto' 
              }}>
                {selectedRecord.raw_content}
              </pre>
            </div>
          </div>
        ) : (
          <pre>{selectedRecord ? JSON.stringify(selectedRecord, null, 2) : ''}</pre>
        )}
      </Drawer>

      <Modal
        title="Store New JSON"
        open={modalOpen}
        onOk={handleStore}
        onCancel={() => setModalOpen(false)}
      >
        <TextArea
          rows={10}
          value={jsonInput}
          onChange={(e) => setJsonInput(e.target.value)}
          placeholder="Paste your JSON here..."
        />
      </Modal>
    </div>
  );
};

export default OAManager;
