import React, { useState } from 'react';
import { Form, Input, Button, Card, Typography, message, Radio } from 'antd';
import { PlayCircleOutlined } from '@ant-design/icons';
import { triggerTestFlow } from '../services/api';

const { Title, Paragraph } = Typography;

const JenkinsTrigger: React.FC = () => {
  const [loading, setLoading] = useState(false);

  const onFinish = async (values: any) => {
    setLoading(true);
    try {
      await triggerTestFlow(values);
      message.success('Test flow triggered! Check Feishu for the card.');
    } catch (error) {
      message.error('Failed to trigger flow');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div style={{ padding: '24px', maxWidth: '600px', margin: '0 auto' }}>
      <Title level={2}>Jenkins Test Flow Trigger</Title>
      <Paragraph>
        This tool simulates the full flow: 
        <br />
        1. Fetch latest OA data
        <br />
        2. <strong>Create Feishu Group</strong> (based on OA Initiator)
        <br />
        3. Add Initiator to the group
        <br />
        4. Generate & Send Feishu Card to the new group
      </Paragraph>
      
      <Card title="Configuration">
        <Form
          layout="vertical"
          onFinish={onFinish}
          initialValues={{
            receive_id_type: 'open_id'
          }}
        >
          <Form.Item
            name="receive_id"
            label="Debug Receiver ID (Feishu)"
            tooltip="Progress logs will be sent here. The final card will be sent to the created group."
            rules={[{ required: true, message: 'Please input receive ID for logs!' }]}
          >
            <Input placeholder="Enter your Open ID/User ID to receive logs" />
          </Form.Item>

          <Form.Item
            name="receive_id_type"
            label="ID Type"
            rules={[{ required: true }]}
          >
            <Radio.Group>
              <Radio value="open_id">Open ID</Radio>
              <Radio value="user_id">User ID</Radio>
              <Radio value="chat_id">Chat ID</Radio>
            </Radio.Group>
          </Form.Item>

          <div style={{ marginBottom: 16, color: '#faad14' }}>
            Note: The group will be created with the OA Request's <strong>Initiator</strong>. 
            If you are not the initiator, you may not see the created group/card.
            Ensure the latest OA data has a valid initiator name that matches a Feishu user.
          </div>

          <Form.Item>
            <Button type="primary" htmlType="submit" icon={<PlayCircleOutlined />} loading={loading} block size="large">
              Trigger Flow
            </Button>
          </Form.Item>
        </Form>
      </Card>
    </div>
  );
};

export default JenkinsTrigger;
